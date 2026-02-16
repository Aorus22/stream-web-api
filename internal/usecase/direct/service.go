package direct

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

	"torrent-stream/internal/domain"
	"torrent-stream/internal/infrastructure/persistence"
)

type Service struct {
	repo     *persistence.DirectDownloadRepository
	cacheDir string

	mu            sync.RWMutex
	active        map[int]*downloadTask
	subscribers   map[int]map[chan domain.DownloadProgress]struct{}
	lastBroadcast map[int]domain.DownloadProgress

	onDemandMu     sync.Mutex
	onDemand       map[int]*onDemandState
	prefetchMu     sync.Mutex
	prefetchCancel map[int]context.CancelFunc
}

const (
	prefetchChunkSize = 4 * 1024 * 1024
	prefetchDelay     = 500 * time.Millisecond
)

type onDemandState struct {
	fileMu     sync.Mutex
	mu         sync.Mutex
	ranges     intervalSet
	totalBytes int64
	mimeType   string
}

type interval struct {
	start int64 // inclusive
	end   int64 // exclusive
}

type intervalSet struct {
	list []interval
}

func (s *intervalSet) add(start, end int64) {
	if start < 0 || end <= start {
		return
	}
	in := interval{start: start, end: end}
	var out []interval
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
		// overlap/adjacent -> merge
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

func (s *intervalSet) covers(start, end int64) bool {
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

func (s *intervalSet) totalLen() int64 {
	var sum int64
	for _, cur := range s.list {
		sum += cur.end - cur.start
	}
	return sum
}

func (s *intervalSet) highest() int64 {
	var max int64
	for _, cur := range s.list {
		if cur.end > max {
			max = cur.end
		}
	}
	return max
}

func NewService(repo *persistence.DirectDownloadRepository, cacheDir string) (*Service, error) {
	if repo == nil {
		return nil, errors.New("repo required")
	}
	if cacheDir == "" {
		return nil, errors.New("cacheDir required")
	}
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}

	return &Service{
		repo:           repo,
		cacheDir:       cacheDir,
		active:         make(map[int]*downloadTask),
		subscribers:    make(map[int]map[chan domain.DownloadProgress]struct{}),
		lastBroadcast:  make(map[int]domain.DownloadProgress),
		onDemand:       make(map[int]*onDemandState),
		prefetchCancel: make(map[int]context.CancelFunc),
	}, nil
}

func (s *Service) AddDownload(downloadURL string) (*domain.DirectDownload, error) {
	u, err := url.Parse(downloadURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http/https urls supported")
	}

	filename := guessFilenameFromURL(u)
	filename = sanitizeFilename(filename)
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
	task := &downloadTask{
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

	go s.runTask(task)

	return s.GetDownload(id)
}

// AddOnDemand creates a record and an empty cache file. Ranges are downloaded on-demand
// based on the player's HTTP Range requests.
func (s *Service) AddOnDemand(downloadURL string) (*domain.DirectDownload, error) {
	u, err := url.Parse(downloadURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, errors.New("invalid url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("only http/https urls supported")
	}

	filename := guessFilenameFromURL(u)
	filename = sanitizeFilename(filename)
	if filename == "" {
		filename = fmt.Sprintf("stream_%d.bin", time.Now().Unix())
	}

	uniqueSuffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	filePath := filepath.Join(s.cacheDir, fmt.Sprintf("%s_%s", uniqueSuffix, filename))

	// Create empty file so WriteAt works and cache path exists.
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

	s.broadcast(domain.DownloadProgress{
		ID:              id,
		Progress:        0,
		DownloadedBytes: 0,
		TotalBytes:      0,
		Status:          "on_demand",
	})

	return s.GetDownload(id)
}

func (s *Service) OnDemandIsCached(id int, start, end int64) bool {
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

func (s *Service) OnDemandAcquireFileLock(id int) func() {
	s.onDemandMu.Lock()
	st, ok := s.onDemand[id]
	if !ok {
		st = &onDemandState{}
		s.onDemand[id] = st
	}
	s.onDemandMu.Unlock()

	st.fileMu.Lock()
	return func() { st.fileMu.Unlock() }
}

func (s *Service) OnDemandRecordRange(id int, start int64, end int64, totalBytes int64, mimeType string) {
	s.onDemandMu.Lock()
	st, ok := s.onDemand[id]
	if !ok {
		st = &onDemandState{}
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

	progress := computeProgress(downloaded, total, "downloading")
	_ = s.repo.UpdateProgress(id, progress, downloaded, total)
	s.broadcast(domain.DownloadProgress{
		ID:              id,
		Progress:        progress,
		DownloadedBytes: downloaded,
		TotalBytes:      total,
		Status:          "on_demand",
	})
}

func (s *Service) getHighestCached(id int) int64 {
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

func (s *Service) StartBackgroundPrefetch(id int) {
	s.prefetchMu.Lock()
	if _, ok := s.prefetchCancel[id]; ok {
		s.prefetchMu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.prefetchCancel[id] = cancel
	s.prefetchMu.Unlock()

	go s.backgroundPrefetchLoop(id, ctx)
}

func (s *Service) StopBackgroundPrefetch(id int) {
	s.prefetchMu.Lock()
	if cancel, ok := s.prefetchCancel[id]; ok {
		cancel()
		delete(s.prefetchCancel, id)
	}
	s.prefetchMu.Unlock()
}

func (s *Service) backgroundPrefetchLoop(id int, ctx context.Context) {
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

		end := start + prefetchChunkSize - 1
		if total > 0 && end >= total {
			end = total - 1
		}

		if start > end { // no more to fetch yet
			time.Sleep(prefetchDelay)
			continue
		}

		if s.OnDemandIsCached(id, start, end+1) {
			time.Sleep(prefetchDelay)
			continue
		}

		if err := s.fetchRange(ctx, id, start, end); err != nil {
			time.Sleep(prefetchDelay)
			continue
		}
	}
}

func (s *Service) fetchRange(ctx context.Context, id int, start, end int64) error {
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

	writer := &rangeWriter{f: file, off: start}
	n, err := io.Copy(writer, resp.Body)
	if err != nil {
		return err
	}

	total := download.TotalBytes
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		if _, _, t, ok := parseContentRange(cr); ok && t > 0 {
			total = t
		}
	}
	if total <= 0 && resp.ContentLength > 0 {
		total = resp.ContentLength + start
	}

	s.OnDemandRecordRange(id, start, start+n, total, resp.Header.Get("Content-Type"))
	return nil
}

type rangeWriter struct {
	f   *os.File
	off int64
}

func (w *rangeWriter) Write(p []byte) (int, error) {
	n, err := w.f.WriteAt(p, w.off)
	if err == nil {
		w.off += int64(n)
	}
	return n, err
}

func parseContentRange(cr string) (int64, int64, int64, bool) {
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

func (s *Service) runTask(task *downloadTask) {
	id := task.id
	filePath := task.filePath

	client := grab.NewClient()
	req, err := grab.NewRequest(filePath, task.url)
	if err != nil {
		s.finishFailed(id, filePath)
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
			s.finishFailed(id, filePath)
			return
		case <-ticker.C:
			s.updateProgressFromResp(id, resp, "downloading")
		case <-resp.Done:
			s.updateProgressFromResp(id, resp, "downloading")
			if err := resp.Err(); err != nil {
				s.finishFailed(id, filePath)
				return
			}
			s.finishSuccess(id, filePath)
			return
		}
	}
}

func (s *Service) updateProgressFromResp(id int, resp *grab.Response, status string) {
	progress := computeProgress(resp.BytesComplete(), resp.Size(), status)
	downloaded := resp.BytesComplete()
	total := resp.Size()
	if total <= 0 {
		total = downloaded
	}
	_ = s.repo.UpdateProgress(id, progress, downloaded, total)
	s.broadcast(domain.DownloadProgress{
		ID:              id,
		Progress:        progress,
		DownloadedBytes: downloaded,
		TotalBytes:      total,
		Status:          status,
	})
}

func (s *Service) finishFailed(id int, filePath string) {
	s.mu.Lock()
	delete(s.active, id)
	s.mu.Unlock()
	s.StopBackgroundPrefetch(id)
	_ = s.repo.MarkFailed(id)
	_ = os.Remove(filePath)
	s.broadcast(domain.DownloadProgress{
		ID:              id,
		Progress:        0,
		DownloadedBytes: 0,
		TotalBytes:      0,
		Status:          "failed",
	})
}

func (s *Service) finishSuccess(id int, filePath string) {
	s.mu.Lock()
	delete(s.active, id)
	s.mu.Unlock()
	s.StopBackgroundPrefetch(id)
	var size int64
	if info, statErr := os.Stat(filePath); statErr == nil {
		size = info.Size()
	}
	_ = s.repo.MarkCompleted(id, filePath, size)
	s.broadcast(domain.DownloadProgress{
		ID:              id,
		Progress:        100,
		DownloadedBytes: size,
		TotalBytes:      size,
		Status:          "completed",
	})
}

func (s *Service) GetDownload(id int) (*domain.DirectDownload, error) {
	return s.repo.Get(id)
}

func (s *Service) ListDownloads() ([]domain.DirectDownload, error) {
	return s.repo.List()
}

func (s *Service) CancelDownload(id int) error {
	s.mu.RLock()
	task := s.active[id]
	s.mu.RUnlock()
	if task == nil {
		return errors.New("download not active")
	}
	task.cancel()
	return nil
}

func (s *Service) DeleteDownload(id int) error {
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

func (s *Service) DeleteAll() error {
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
	s.onDemand = make(map[int]*onDemandState)
	s.onDemandMu.Unlock()
	return s.repo.DeleteAll()
}

func (s *Service) StreamProgress(ctx context.Context, id int) <-chan domain.DownloadProgress {
	ch := make(chan domain.DownloadProgress, 8)

	s.mu.Lock()
	if _, ok := s.subscribers[id]; !ok {
		s.subscribers[id] = make(map[chan domain.DownloadProgress]struct{})
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

func (s *Service) broadcast(p domain.DownloadProgress) {
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

func computeProgress(downloadedBytes int64, totalBytes int64, status string) float64 {
	if status == "completed" {
		return 100
	}
	if totalBytes <= 0 {
		return 0
	}
	return (float64(downloadedBytes) / float64(totalBytes)) * 100
}

func guessFilenameFromURL(u *url.URL) string {
	base := filepath.Base(u.Path)
	base = strings.TrimSpace(base)
	if base == "." || base == "/" {
		return ""
	}
	return base
}

var invalidFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)

func sanitizeFilename(name string) string {
	name = invalidFilenameChars.ReplaceAllString(name, "_")
	name = strings.TrimSpace(name)
	if len(name) > 200 {
		name = name[:200]
	}
	return name
}
