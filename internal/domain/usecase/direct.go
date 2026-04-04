package usecase

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cavaliergopher/grab/v3"

	"stream-web-api/internal/domain/model"
	domainrepo "stream-web-api/internal/domain/repository"
)

type DirectDownloadUsecase struct {
	repo     domainrepo.DirectDownloadRepository
	cacheDir string

	mu            sync.RWMutex
	active        map[int]*directDownloadTask
	subscribers   map[int]map[chan model.DownloadProgress]struct{}
	lastBroadcast map[int]model.DownloadProgress

	onDemandMu     sync.Mutex
	onDemand       map[int]*directOnDemandState
	prefetchMu     sync.Mutex
	prefetchCancel map[int]context.CancelFunc
}

const (
	directPrefetchChunkSize = 4 * 1024 * 1024
	directPrefetchDelay     = 500 * time.Millisecond
)

type directDownloadTask struct {
	id       int
	url      string
	filePath string

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func (t *directDownloadTask) run(cb func(downloadedBytes int64, totalBytes int64, status string)) error {
	defer close(t.done)

	req, err := http.NewRequestWithContext(t.ctx, http.MethodGet, t.url, nil)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 0,
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		cb(0, 0, "failed")
		return &directHTTPError{statusCode: resp.StatusCode}
	}

	totalBytes := resp.ContentLength
	if totalBytes < 0 {
		totalBytes = 0
	}

	out, err := os.Create(t.filePath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	buf := make([]byte, 1024*256)
	var downloaded int64
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := out.Write(buf[:n]); err != nil {
				return err
			}
			downloaded += int64(n)
		}

		if time.Since(lastReport) >= 500*time.Millisecond {
			cb(downloaded, totalBytes, "downloading")
			lastReport = time.Now()
		}

		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return readErr
		}
	}

	cb(downloaded, totalBytes, "completed")
	return nil
}

type directHTTPError struct {
	statusCode int
}

func (e *directHTTPError) Error() string {
	return http.StatusText(e.statusCode)
}

type directOnDemandState struct {
	fileMu     sync.Mutex
	mu         sync.Mutex
	ranges     directIntervalSet
	totalBytes int64
	mimeType   string
}

type directInterval struct {
	start int64
	end   int64
}

type directIntervalSet struct {
	list []directInterval
}

func (s *directIntervalSet) add(start, end int64) {
	if start < 0 || end <= start {
		return
	}
	in := directInterval{start: start, end: end}
	var out []directInterval
	inserted := false

	for _, cur := range s.list {
		if cur.end < in.start {
			out = append(out, cur)
			continue
		}
		if in.end < cur.start {
			if !inserted {
				out = append(out, in)
				inserted = true
			}
			out = append(out, cur)
			continue
		}
		if cur.start < in.start {
			in.start = cur.start
		}
		if cur.end > in.end {
			in.end = cur.end
		}
	}

	if !inserted {
		out = append(out, in)
	}
	s.list = out
}

func (s *directIntervalSet) covers(start, end int64) bool {
	if start < 0 || end <= start {
		return false
	}
	for _, cur := range s.list {
		if start >= cur.start && end <= cur.end {
			return true
		}
		if cur.start > start {
			return false
		}
	}
	return false
}

func (s *directIntervalSet) totalLen() int64 {
	var sum int64
	for _, cur := range s.list {
		sum += cur.end - cur.start
	}
	return sum
}

func (s *directIntervalSet) highest() int64 {
	var max int64
	for _, cur := range s.list {
		if cur.end > max {
			max = cur.end
		}
	}
	return max
}

type directRangeWriter struct {
	f   *os.File
	off int64
}

func (w *directRangeWriter) Write(p []byte) (int, error) {
	n, err := w.f.WriteAt(p, w.off)
	if err == nil {
		w.off += int64(n)
	}
	return n, err
}

func NewDirectDownloadUsecase(repo domainrepo.DirectDownloadRepository, cacheDir string) (*DirectDownloadUsecase, error) {
	if repo == nil {
		return nil, errors.New("repo required")
	}
	if cacheDir == "" {
		return nil, errors.New("cacheDir required")
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	return &DirectDownloadUsecase{
		repo:           repo,
		cacheDir:       cacheDir,
		active:         make(map[int]*directDownloadTask),
		subscribers:    make(map[int]map[chan model.DownloadProgress]struct{}),
		lastBroadcast:  make(map[int]model.DownloadProgress),
		onDemand:       make(map[int]*directOnDemandState),
		prefetchCancel: make(map[int]context.CancelFunc),
	}, nil
}

func (s *DirectDownloadUsecase) AddDownload(downloadURL string) (*model.DirectDownload, error) {
	u, err := url.Parse(downloadURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http/https urls supported")
	}

	filename := directGuessFilenameFromURL(u)
	filename = directSanitizeFilename(filename)
	if filename == "" {
		filename = fmt.Sprintf("download_%d.bin", time.Now().Unix())
	}

	uniqueSuffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	filePath := filepath.Join(s.cacheDir, fmt.Sprintf("%s_%s", uniqueSuffix, filename))

	id, err := s.repo.Create(downloadURL, filename, "downloading", filePath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &directDownloadTask{
		id:       id,
		url:      downloadURL,
		filePath: filePath,
		ctx:      ctx,
		cancel:   cancel,
		done:     make(chan struct{}),
	}

	s.mu.Lock()
	s.active[id] = task
	s.mu.Unlock()

	go s.directRunTask(task)

	return s.GetDownload(id)
}

func (s *DirectDownloadUsecase) AddOnDemand(downloadURL string) (*model.DirectDownload, error) {
	u, err := url.Parse(downloadURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http/https urls supported")
	}

	filename := directGuessFilenameFromURL(u)
	filename = directSanitizeFilename(filename)
	if filename == "" {
		filename = fmt.Sprintf("stream_%d.bin", time.Now().Unix())
	}

	uniqueSuffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	filePath := filepath.Join(s.cacheDir, fmt.Sprintf("%s_%s", uniqueSuffix, filename))

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	_ = f.Close()

	id, err := s.repo.Create(downloadURL, filename, "on_demand", filePath)
	if err != nil {
		_ = os.Remove(filePath)
		return nil, err
	}

	s.directBroadcast(model.DownloadProgress{
		ID:              id,
		Progress:        0,
		DownloadedBytes: 0,
		TotalBytes:      0,
		Status:          "on_demand",
	})

	return s.GetDownload(id)
}

func (s *DirectDownloadUsecase) AddWithMode(downloadURL string, mode string) (*model.DirectDownload, error) {
	normalizedMode := strings.ToLower(strings.TrimSpace(mode))
	if normalizedMode == "ondemand" || normalizedMode == "on_demand" || normalizedMode == "stream" {
		return s.AddOnDemand(downloadURL)
	}
	return s.AddDownload(downloadURL)
}

func (s *DirectDownloadUsecase) OnDemandIsCached(id int, start, end int64) bool {
	s.onDemandMu.Lock()
	st, ok := s.onDemand[id]
	if !ok {
		s.onDemandMu.Unlock()
		return false
	}
	s.onDemandMu.Unlock()

	st.mu.Lock()
	defer st.mu.Unlock()
	return st.ranges.covers(start, end)
}

func (s *DirectDownloadUsecase) OnDemandAcquireFileLock(id int) func() {
	s.onDemandMu.Lock()
	st, ok := s.onDemand[id]
	if !ok {
		st = &directOnDemandState{}
		s.onDemand[id] = st
	}
	s.onDemandMu.Unlock()

	st.fileMu.Lock()
	return func() { st.fileMu.Unlock() }
}

func (s *DirectDownloadUsecase) OnDemandRecordRange(id int, start int64, end int64, totalBytes int64, mimeType string) {
	s.onDemandMu.Lock()
	st, ok := s.onDemand[id]
	if !ok {
		st = &directOnDemandState{}
		s.onDemand[id] = st
	}
	s.onDemandMu.Unlock()

	st.mu.Lock()
	st.ranges.add(start, end)
	if totalBytes > 0 && st.totalBytes <= 0 {
		st.totalBytes = totalBytes
	}
	if mimeType != "" && st.mimeType == "" {
		st.mimeType = mimeType
	}
	downloaded := st.ranges.totalLen()
	total := st.totalBytes
	st.mu.Unlock()

	progress := directComputeProgress(downloaded, total, "downloading")
	_ = s.repo.UpdateProgress(id, progress, downloaded, total)
	s.directBroadcast(model.DownloadProgress{
		ID:              id,
		Progress:        progress,
		DownloadedBytes: downloaded,
		TotalBytes:      total,
		Status:          "on_demand",
	})
}

func (s *DirectDownloadUsecase) getHighestCached(id int) int64 {
	s.onDemandMu.Lock()
	st, ok := s.onDemand[id]
	s.onDemandMu.Unlock()
	if !ok {
		return 0
	}

	st.mu.Lock()
	defer st.mu.Unlock()
	return st.ranges.highest()
}

func (s *DirectDownloadUsecase) StartBackgroundPrefetch(id int) {
	s.prefetchMu.Lock()
	if _, ok := s.prefetchCancel[id]; ok {
		s.prefetchMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.prefetchCancel[id] = cancel
	s.prefetchMu.Unlock()

	go s.directBackgroundPrefetchLoop(id, ctx)
}

func (s *DirectDownloadUsecase) StopBackgroundPrefetch(id int) {
	s.prefetchMu.Lock()
	if cancel, ok := s.prefetchCancel[id]; ok {
		cancel()
		delete(s.prefetchCancel, id)
	}
	s.prefetchMu.Unlock()
}

func (s *DirectDownloadUsecase) directBackgroundPrefetchLoop(id int, ctx context.Context) {
	defer func() {
		s.prefetchMu.Lock()
		delete(s.prefetchCancel, id)
		s.prefetchMu.Unlock()
	}()

	for {
		if ctx.Err() != nil {
			return
		}

		download, err := s.repo.Get(id)
		if err != nil {
			return
		}
		if download.Status != "on_demand" {
			return
		}

		start := s.getHighestCached(id)
		total := download.TotalBytes
		if total > 0 && start >= total {
			return
		}

		end := start + directPrefetchChunkSize - 1
		if total > 0 && end >= total {
			end = total - 1
		}

		if start > end {
			time.Sleep(directPrefetchDelay)
			continue
		}

		if s.OnDemandIsCached(id, start, end+1) {
			time.Sleep(directPrefetchDelay)
			continue
		}

		if err := s.directFetchRange(ctx, id, start, end); err != nil {
			time.Sleep(directPrefetchDelay)
			continue
		}
	}
}

func (s *DirectDownloadUsecase) directFetchRange(ctx context.Context, id int, start, end int64) error {
	download, err := s.repo.Get(id)
	if err != nil {
		return err
	}
	if download.FilePath == "" || download.URL == "" {
		return errors.New("missing download metadata")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, download.URL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
			return nil
		}
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	release := s.OnDemandAcquireFileLock(id)
	defer release()

	file, err := os.OpenFile(download.FilePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := &directRangeWriter{f: file, off: start}
	n, err := io.Copy(writer, resp.Body)
	if err != nil {
		return err
	}

	total := download.TotalBytes
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		if _, _, t, ok := directParseContentRange(cr); ok && t > 0 {
			total = t
		}
	}
	if total <= 0 && resp.ContentLength > 0 {
		total = resp.ContentLength + start
	}

	s.OnDemandRecordRange(id, start, start+n, total, resp.Header.Get("Content-Type"))
	return nil
}

func directParseContentRange(cr string) (int64, int64, int64, bool) {
	if !strings.HasPrefix(cr, "bytes ") {
		return 0, 0, 0, false
	}
	cr = strings.TrimPrefix(cr, "bytes ")
	parts := strings.Split(cr, "/")
	if len(parts) != 2 {
		return 0, 0, 0, false
	}
	rangePart := parts[0]
	totalPart := parts[1]
	border := strings.Split(rangePart, "-")
	if len(border) != 2 {
		return 0, 0, 0, false
	}
	start, err := strconv.ParseInt(strings.TrimSpace(border[0]), 10, 64)
	if err != nil {
		return 0, 0, 0, false
	}
	end, err := strconv.ParseInt(strings.TrimSpace(border[1]), 10, 64)
	if err != nil {
		return 0, 0, 0, false
	}
	total := int64(0)
	if totalPart != "*" {
		if parsed, err := strconv.ParseInt(strings.TrimSpace(totalPart), 10, 64); err == nil {
			total = parsed
		}
	}
	return start, end, total, true
}

func (s *DirectDownloadUsecase) directRunTask(task *directDownloadTask) {
	id := task.id
	filePath := task.filePath

	client := grab.NewClient()
	req, err := grab.NewRequest(filePath, task.url)
	if err != nil {
		s.directFinishFailed(id, filePath)
		return
	}
	req = req.WithContext(task.ctx)

	resp := client.Do(req)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-task.ctx.Done():
			resp.Cancel()
			s.directFinishFailed(id, filePath)
			return
		case <-ticker.C:
			s.directUpdateProgressFromResp(id, resp, "downloading")
		case <-resp.Done:
			s.directUpdateProgressFromResp(id, resp, "downloading")
			if err := resp.Err(); err != nil {
				s.directFinishFailed(id, filePath)
				return
			}
			s.directFinishSuccess(id, filePath)
			return
		}
	}
}

func (s *DirectDownloadUsecase) directUpdateProgressFromResp(id int, resp *grab.Response, status string) {
	progress := directComputeProgress(resp.BytesComplete(), resp.Size(), status)
	downloaded := resp.BytesComplete()
	total := resp.Size()
	if total <= 0 {
		total = downloaded
	}
	_ = s.repo.UpdateProgress(id, progress, downloaded, total)
	s.directBroadcast(model.DownloadProgress{
		ID:              id,
		Progress:        progress,
		DownloadedBytes: downloaded,
		TotalBytes:      total,
		Status:          status,
	})
}

func (s *DirectDownloadUsecase) directFinishFailed(id int, filePath string) {
	s.mu.Lock()
	delete(s.active, id)
	s.mu.Unlock()
	s.StopBackgroundPrefetch(id)
	_ = s.repo.MarkFailed(id)
	_ = os.Remove(filePath)
	s.directBroadcast(model.DownloadProgress{
		ID:              id,
		Progress:        0,
		DownloadedBytes: 0,
		TotalBytes:      0,
		Status:          "failed",
	})
}

func (s *DirectDownloadUsecase) directFinishSuccess(id int, filePath string) {
	s.mu.Lock()
	delete(s.active, id)
	s.mu.Unlock()
	s.StopBackgroundPrefetch(id)
	var size int64
	if info, statErr := os.Stat(filePath); statErr == nil {
		size = info.Size()
	}
	_ = s.repo.MarkCompleted(id, filePath, size)
	s.directBroadcast(model.DownloadProgress{
		ID:              id,
		Progress:        100,
		DownloadedBytes: size,
		TotalBytes:      size,
		Status:          "completed",
	})
}

func directToDomainDownload(m *model.DirectDownload) *model.DirectDownload {
	if m == nil {
		return nil
	}
	return &model.DirectDownload{
		ID:              m.ID,
		URL:             m.URL,
		Filename:        m.Filename,
		Status:          m.Status,
		Progress:        m.Progress,
		DownloadedBytes: m.DownloadedBytes,
		TotalBytes:      m.TotalBytes,
		AddedAt:         m.AddedAt,
		CompletedAt:     m.CompletedAt,
		FilePath:        m.FilePath,
	}
}

func directToDomainDownloads(ms []model.DirectDownload) []model.DirectDownload {
	result := make([]model.DirectDownload, len(ms))
	for i, m := range ms {
		result[i] = *directToDomainDownload(&m)
	}
	return result
}

func (s *DirectDownloadUsecase) GetDownload(id int) (*model.DirectDownload, error) {
	m, err := s.repo.Get(id)
	if err != nil {
		return nil, err
	}
	return directToDomainDownload(m), nil
}

func (s *DirectDownloadUsecase) ListDownloads() ([]model.DirectDownload, error) {
	ms, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	return directToDomainDownloads(ms), nil
}

func (s *DirectDownloadUsecase) CancelDownload(id int) error {
	s.mu.RLock()
	task := s.active[id]
	s.mu.RUnlock()
	if task == nil {
		return errors.New("download not active")
	}
	task.cancel()
	return nil
}

func (s *DirectDownloadUsecase) DeleteDownload(id int) error {
	dl, err := s.repo.Get(id)
	if err != nil {
		return err
	}

	_ = s.CancelDownload(id)

	if dl.FilePath != "" {
		_ = os.Remove(dl.FilePath)
	}

	s.StopBackgroundPrefetch(id)

	s.onDemandMu.Lock()
	delete(s.onDemand, id)
	s.onDemandMu.Unlock()

	return s.repo.Delete(id)
}

func (s *DirectDownloadUsecase) DeleteAll() error {
	dls, err := s.repo.List()
	if err == nil {
		for _, dl := range dls {
			s.StopBackgroundPrefetch(dl.ID)
			_ = s.CancelDownload(dl.ID)
			if dl.FilePath != "" {
				_ = os.Remove(dl.FilePath)
			}
		}
	}
	s.onDemandMu.Lock()
	s.onDemand = make(map[int]*directOnDemandState)
	s.onDemandMu.Unlock()
	return s.repo.DeleteAll()
}

func (s *DirectDownloadUsecase) StreamProgress(ctx context.Context, id int) <-chan model.DownloadProgress {
	ch := make(chan model.DownloadProgress, 8)

	s.mu.Lock()
	if _, ok := s.subscribers[id]; !ok {
		s.subscribers[id] = make(map[chan model.DownloadProgress]struct{})
	}
	s.subscribers[id][ch] = struct{}{}
	if last, ok := s.lastBroadcast[id]; ok {
		ch <- last
	}
	s.mu.Unlock()

	go func() {
		<-ctx.Done()
		s.mu.Lock()
		if subs, ok := s.subscribers[id]; ok {
			delete(subs, ch)
			if len(subs) == 0 {
				delete(s.subscribers, id)
			}
		}
		s.mu.Unlock()
		close(ch)
	}()

	return ch
}

func (s *DirectDownloadUsecase) directBroadcast(p model.DownloadProgress) {
	s.mu.Lock()
	s.lastBroadcast[p.ID] = p
	subs := s.subscribers[p.ID]
	for ch := range subs {
		select {
		case ch <- p:
		default:
		}
	}
	s.mu.Unlock()
}

func directComputeProgress(downloadedBytes int64, totalBytes int64, status string) float64 {
	if status == "completed" {
		return 100
	}
	if totalBytes <= 0 {
		return 0
	}
	return (float64(downloadedBytes) / float64(totalBytes)) * 100
}

func directGuessFilenameFromURL(u *url.URL) string {
	base := filepath.Base(u.Path)
	base = strings.TrimSpace(base)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

var directInvalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)

func directSanitizeFilename(name string) string {
	name = directInvalidFilenameChars.ReplaceAllString(name, "_")
	name = strings.TrimSpace(name)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}
