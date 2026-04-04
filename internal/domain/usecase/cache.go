package usecase

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"stream-web-api/internal/domain/model"
	domainrepo "stream-web-api/internal/domain/repository"
)

type cacheGdriveJob struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
	Progress float64 `json:"progress"`
	Link     string `json:"link,omitempty"`
	Error    string `json:"error,omitempty"`
	cancel   context.CancelFunc
}

type CacheUsecase struct {
	cacheDir       string
	directCacheDir string
	hlsCacheDir    string
	torrentService *TorrentUsecase
	directService  *DirectDownloadUsecase
	gdriveClient   domainrepo.GDriveClient
	streamService  *StreamUsecase

	gdriveJobs sync.Map
}

func NewCacheUsecase(
	cacheDir string,
	directCacheDir string,
	hlsCacheDir string,
	torrentService *TorrentUsecase,
	directService *DirectDownloadUsecase,
	gdriveClient domainrepo.GDriveClient,
) *CacheUsecase {
	return &CacheUsecase{
		cacheDir:       cacheDir,
		directCacheDir: directCacheDir,
		hlsCacheDir:    hlsCacheDir,
		torrentService: torrentService,
		directService:  directService,
		gdriveClient:   gdriveClient,
	}
}

func (s *CacheUsecase) SetStreamService(ss *StreamUsecase) {
	s.streamService = ss
}

func (s *CacheUsecase) GetReencodeJobs() []model.ReencodeJobStatus {
	if s.streamService == nil {
		return nil
	}
	return s.streamService.GetReencodeJobs()
}

func (s *CacheUsecase) GetGDriveJobs() []model.GDriveJobStatus {
	var result []model.GDriveJobStatus
	s.gdriveJobs.Range(func(key, value interface{}) bool {
		if job, ok := value.(*cacheGdriveJob); ok {
			result = append(result, model.GDriveJobStatus{
				ID:       job.ID,
				Filename: job.Filename,
				Status:   job.Status,
				Progress: job.Progress,
				Link:     job.Link,
				Error:    job.Error,
			})
		}
		return true
	})
	return result
}

func (s *CacheUsecase) GetTasksEvent() *model.TasksSSEEvent {
	gdriveActive := make([]model.GDriveJob, 0)
	s.gdriveJobs.Range(func(key, value interface{}) bool {
		if job, ok := value.(*cacheGdriveJob); ok {
			gdriveActive = append(gdriveActive, model.GDriveJob{
				ID: job.ID, Filename: job.Filename, Status: job.Status,
				Progress: job.Progress, Link: job.Link, Error: job.Error,
			})
		}
		return true
	})

	reencodeJobs := s.GetReencodeJobs()
	reencodeActive := make([]interface{}, 0, len(reencodeJobs))
	for _, job := range reencodeJobs {
		reencodeActive = append(reencodeActive, job)
	}

	return &model.TasksSSEEvent{
		GDrive:   gdriveActive,
		Reencode: reencodeActive,
	}
}

func (s *CacheUsecase) ResolveExportPath(relPath string) (string, error) {
	cleanPath := filepath.Clean(strings.TrimPrefix(relPath, "/"))
	fullPath := filepath.Join(s.cacheDir, "exports", cleanPath)
	if !strings.HasPrefix(fullPath, filepath.Join(s.cacheDir, "exports")) {
		return "", fmt.Errorf("access denied")
	}
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return "", fmt.Errorf("export file not found")
	}
	return fullPath, nil
}

func cacheIsVideoFile(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	videoExtensions := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts", ".m2ts"}
	for _, ve := range videoExtensions {
		if ext == ve {
			return true
		}
	}
	return false
}

func cacheIsPossibleInfoHash(s string) bool {
	if len(s) != 40 {
		return false
	}
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func cacheFindFileIndex(files []model.File, fileName string) int {
	for _, f := range files {
		if f.Name == fileName {
			return f.Index
		}
	}
	return 0
}

func (s *CacheUsecase) ListCachedFiles() ([]model.CachedFileWithType, error) {
	var cachedFiles []model.CachedFileWithType

	torrents := s.torrentService.ListTorrents()
	nameToInfoHash := make(map[string]string)
	infoHashToFiles := make(map[string][]model.File)
	fileNameToTorrent := make(map[string]map[string]string)

	for _, t := range torrents {
		nameToInfoHash[t.Name] = t.InfoHash
		infoHashToFiles[t.InfoHash] = t.Files
		for _, f := range t.Files {
			fileNameToTorrent[f.Name] = map[string]string{
				"infoHash":  t.InfoHash,
				"fileIndex": fmt.Sprintf("%d", f.Index),
			}
		}
	}

	_ = filepath.Walk(s.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if s.directCacheDir != "" && filepath.Clean(path) == filepath.Clean(s.directCacheDir) {
				return filepath.SkipDir
			}
			if filepath.Base(path) == "exports" {
				return filepath.SkipDir
			}
			return nil
		}
		if !cacheIsVideoFile(info.Name()) {
			return nil
		}

		relPath, _ := filepath.Rel(s.cacheDir, path)
		relPath = filepath.ToSlash(relPath)
		parts := strings.Split(relPath, "/")

		var infoHash string
		var fileIndex int
		fileName := info.Name()

		if len(parts) >= 2 {
			folderName := parts[0]
			if cacheIsPossibleInfoHash(folderName) {
				infoHash = folderName
				if files, ok := infoHashToFiles[infoHash]; ok {
					fileIndex = cacheFindFileIndex(files, fileName)
				}
			} else if hash, ok := nameToInfoHash[folderName]; ok {
				infoHash = hash
				if files, ok := infoHashToFiles[hash]; ok {
					fileIndex = cacheFindFileIndex(files, fileName)
				}
			}
		} else {
			if match, ok := fileNameToTorrent[fileName]; ok {
				infoHash = match["infoHash"]
				if idxStr := match["fileIndex"]; idxStr != "" {
					if idx, err := strconv.Atoi(idxStr); err == nil {
						fileIndex = idx
					}
				}
			}
		}

		streamURL := ""
		canPlay := false
		if infoHash != "" {
			streamURL = fmt.Sprintf("/stream/%s/%d", infoHash, fileIndex)
			canPlay = true
		}

		cachedFiles = append(cachedFiles, model.CachedFileWithType{
			Name:      info.Name(),
			Path:      relPath,
			Size:      info.Size(),
			Type:      "magnet",
			InfoHash:  infoHash,
			FileIndex: fileIndex,
			StreamURL: streamURL,
			CanPlay:   canPlay,
		})
		return nil
	})

	exportDir := filepath.Join(s.cacheDir, "exports")
	_ = filepath.Walk(exportDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !cacheIsVideoFile(info.Name()) {
			return nil
		}
		relPath, _ := filepath.Rel(exportDir, path)
		relPath = filepath.ToSlash(relPath)

		cachedFiles = append(cachedFiles, model.CachedFileWithType{
			Name:      info.Name(),
			Path:      "exports/" + relPath,
			Size:      info.Size(),
			Type:      "export",
			Status:    "completed",
			StreamURL: "/api/exports/" + relPath,
			CanPlay:   false,
		})
		return nil
	})

	if s.directService != nil && s.directCacheDir != "" {
		downloads, err := s.directService.ListDownloads()
		if err == nil {
			byFilePath := make(map[string]struct {
				id       int
				status   string
				progress float64
			})
			for _, dl := range downloads {
				if dl.FilePath == "" {
					continue
				}
				byFilePath[filepath.Clean(dl.FilePath)] = struct {
					id       int
					status   string
					progress float64
				}{
					id:       dl.ID,
					status:   dl.Status,
					progress: dl.Progress,
				}
			}

			_ = filepath.Walk(s.directCacheDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil || info.IsDir() {
					return nil
				}
				if !cacheIsVideoFile(info.Name()) {
					return nil
				}

				relPath, _ := filepath.Rel(s.directCacheDir, path)
				relPath = filepath.ToSlash(relPath)

				rec, ok := byFilePath[filepath.Clean(path)]
				if !ok {
					cachedFiles = append(cachedFiles, model.CachedFileWithType{
						Name:      info.Name(),
						Path:      relPath,
						Size:      info.Size(),
						Type:      "direct",
						Status:    "orphan",
						StreamURL: "",
						CanPlay:   false,
					})
					return nil
				}

				canPlay := rec.status == "completed" || rec.status == "on_demand"
				streamURL := ""
				if rec.id != 0 {
					streamURL = fmt.Sprintf("/stream/direct/%d", rec.id)
				}

				cachedFiles = append(cachedFiles, model.CachedFileWithType{
					Name:       info.Name(),
					Path:       relPath,
					Size:       info.Size(),
					Type:       "direct",
					DownloadID: rec.id,
					Progress:   rec.progress,
					Status:     rec.status,
					StreamURL:  streamURL,
					CanPlay:    canPlay,
				})
				return nil
			})

			for _, dl := range downloads {
				if dl.FilePath == "" {
					continue
				}
				if _, statErr := os.Stat(dl.FilePath); statErr == nil {
					continue
				}
				cachedFiles = append(cachedFiles, model.CachedFileWithType{
					Name:       dl.Filename,
					Path:       dl.FilePath,
					Size:       0,
					Type:       "direct",
					DownloadID: dl.ID,
					Progress:   dl.Progress,
					Status:     "missing",
					StreamURL:  fmt.Sprintf("/stream/direct/%d", dl.ID),
					CanPlay:    false,
				})
			}
		}
	}

	return cachedFiles, nil
}

func (s *CacheUsecase) DeleteCache(infoHash string) error {
	folderPath := filepath.Join(s.cacheDir, infoHash)
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		return fmt.Errorf("cache folder not found")
	}
	if err := os.RemoveAll(folderPath); err != nil {
		return fmt.Errorf("failed to delete cache: %w", err)
	}
	return nil
}

func (s *CacheUsecase) ClearAllCache() error {
	if err := cacheClearDirectory(s.cacheDir, []string{"torrents.db", "torrents.db-journal"}, s.directCacheDir); err != nil {
		log.Printf("Failed to clear torrent cache: %v", err)
	}
	if err := cacheClearDirectory(s.hlsCacheDir, []string{}, ""); err != nil {
		log.Printf("Failed to clear HLS cache: %v", err)
	}
	return nil
}

func (s *CacheUsecase) GetCacheStats() (*model.CacheStats, error) {
	var totalSize int64
	var fileCount int

	filepath.Walk(s.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	return &model.CacheStats{
		TotalSize: totalSize,
		FileCount: fileCount,
		CacheDir:  s.cacheDir,
	}, nil
}

func (s *CacheUsecase) StartGDriveUpload(req *model.GDriveUploadParams) (string, string, error) {
	if s.gdriveClient == nil {
		return "", "", fmt.Errorf("Google Drive integration not configured")
	}

	var filePath string
	var filename string
	var jobID string

	if req.ExportPath != "" {
		cleanPath := filepath.Clean(strings.TrimPrefix(req.ExportPath, "/"))
		filePath = filepath.Join(s.cacheDir, cleanPath)
		filename = filepath.Base(filePath)
		jobID = "export_" + cleanPath
	} else if req.InfoHash != "" {
		stats, err := s.torrentService.GetStats(req.InfoHash)
		if err != nil {
			return "", "", fmt.Errorf("torrent session not active: %w", err)
		}

		var foundFile *model.File
		torrentName := stats.Name
		for i, f := range stats.Files {
			if i == req.FileIndex {
				foundFile = &f
				break
			}
		}

		if foundFile == nil || foundFile.Name == "" {
			return "", "", fmt.Errorf("file not found in torrent")
		}

		filename = foundFile.Name
		jobID = fmt.Sprintf("torrent_%s_%d", req.InfoHash, req.FileIndex)

		candidates := []string{
			filepath.Join(s.cacheDir, torrentName, filename),
			filepath.Join(s.cacheDir, filename),
			filepath.Join(s.cacheDir, req.InfoHash, filename),
		}

		for _, path := range candidates {
			if _, err := os.Stat(path); err == nil {
				filePath = path
				break
			}
		}

		if filePath == "" {
			_ = filepath.Walk(s.cacheDir, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && info.Name() == filepath.Base(filename) {
					filePath = path
					return filepath.SkipDir
				}
				return nil
			})
		}
	} else if req.DownloadID != 0 {
		downloads, err := s.directService.ListDownloads()
		if err == nil {
			for _, dl := range downloads {
				if dl.ID == req.DownloadID {
					filePath = dl.FilePath
					filename = dl.Filename
					jobID = fmt.Sprintf("direct_%d", req.DownloadID)
					break
				}
			}
		}
	}

	if filePath == "" {
		return "", "", fmt.Errorf("file path could not be resolved")
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("file not found on disk: %s", filePath)
	}

	ctx, cancel := context.WithCancel(context.Background())
	job := &cacheGdriveJob{
		ID:       jobID,
		Filename: filename,
		Status:   "uploading",
		cancel:   cancel,
	}
	s.gdriveJobs.Store(jobID, job)

	go func() {
		_, link, err := s.gdriveClient.Upload(ctx, filePath, filename, func(p float64) {
			job.Progress = p
		})

		if err != nil {
			if ctx.Err() == context.Canceled {
				job.Status = "canceled"
			} else {
				job.Status = "failed"
				job.Error = err.Error()
			}
			time.AfterFunc(10*time.Minute, func() { s.gdriveJobs.Delete(jobID) })
		} else {
			job.Status = "completed"
			job.Progress = 100
			job.Link = link
			time.AfterFunc(5*time.Minute, func() { s.gdriveJobs.Delete(jobID) })
		}
	}()

	return filePath, jobID, nil
}

func (s *CacheUsecase) CancelGDriveUpload(jobID string) error {
	if val, ok := s.gdriveJobs.Load(jobID); ok {
		job := val.(*cacheGdriveJob)
		if job.cancel != nil {
			job.cancel()
			return nil
		}
	}
	return fmt.Errorf("job not found or already finished")
}

func cacheClearDirectory(dir string, skipFiles []string, directCacheDir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if directCacheDir != "" && entry.Name() == filepath.Base(directCacheDir) {
			continue
		}
		shouldSkip := false
		for _, skip := range skipFiles {
			if entry.Name() == skip {
				shouldSkip = true
				break
			}
		}
		if shouldSkip {
			continue
		}
		_ = os.RemoveAll(filepath.Join(dir, entry.Name()))
	}
	return nil
}
