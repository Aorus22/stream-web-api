package direct

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

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
		repo:          repo,
		cacheDir:      cacheDir,
		active:        make(map[int]*downloadTask),
		subscribers:   make(map[int]map[chan domain.DownloadProgress]struct{}),
		lastBroadcast: make(map[int]domain.DownloadProgress),
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

func (s *Service) runTask(task *downloadTask) {
	id := task.id
	filePath := task.filePath

	err := task.run(func(downloadedBytes int64, totalBytes int64, status string) {
		progress := computeProgress(downloadedBytes, totalBytes, status)
		_ = s.repo.UpdateProgress(id, progress, downloadedBytes, totalBytes)
		s.broadcast(domain.DownloadProgress{
			ID:              id,
			Progress:        progress,
			DownloadedBytes: downloadedBytes,
			TotalBytes:      totalBytes,
			Status:          status,
		})
	})

	s.mu.Lock()
	delete(s.active, id)
	s.mu.Unlock()

	if err != nil {
		_ = s.repo.MarkFailed(id)
		_ = os.Remove(filePath)
		s.broadcast(domain.DownloadProgress{
			ID:              id,
			Progress:        0,
			DownloadedBytes: 0,
			TotalBytes:      0,
			Status:          "failed",
		})
		return
	}

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

	return s.repo.Delete(id)
}

func (s *Service) DeleteAll() error {
	dls, err := s.repo.List()
	if err == nil {
		for _, dl := range dls {
			_ = s.CancelDownload(dl.ID)
			if dl.FilePath != "" {
				_ = os.Remove(dl.FilePath)
			}
		}
	}
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

