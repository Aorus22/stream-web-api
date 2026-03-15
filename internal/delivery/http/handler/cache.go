package handler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"torrent-stream/internal/infrastructure/gdrive"
	"torrent-stream/internal/domain"
	"torrent-stream/internal/usecase/direct"
	"torrent-stream/internal/usecase/torrent"

	"github.com/gin-gonic/gin"
)

type GDriveJob struct {
	ID       string             `json:"id"`
	Filename string             `json:"filename"`
	Status   string             `json:"status"` // uploading, completed, failed, canceled
	Progress float64            `json:"progress"`
	Link     string             `json:"link,omitempty"`
	Error    string             `json:"error,omitempty"`
	cancel   context.CancelFunc // internal
}

// CacheHandler handles cache-related requests
type CacheHandler struct {
	cacheDir       string
	directCacheDir string
	hlsCacheDir    string
	torrentService *torrent.Service
	directService  *direct.Service
	gdriveClient   *gdrive.Client
	gdriveJobs     sync.Map // map[string]*GDriveJob
	// We'll link reencodeJobs here to merge SSE
	reencodeJobs *sync.Map
}

// NewCacheHandler creates a new cache handler
func NewCacheHandler(cacheDir string, directCacheDir string, hlsCacheDir string, torrentService *torrent.Service, directService *direct.Service, gdriveClient *gdrive.Client) *CacheHandler {
	return &CacheHandler{
		cacheDir:       cacheDir,
		directCacheDir: directCacheDir,
		hlsCacheDir:    hlsCacheDir,
		torrentService: torrentService,
		directService:  directService,
		gdriveClient:   gdriveClient,
	}
}

func (h *CacheHandler) SetReencodeJobs(m *sync.Map) {
	h.reencodeJobs = m
}

// HandleTasksSSE handles GET /api/tasks/stream (Merged Reencode + GDrive)
func (h *CacheHandler) HandleTasksSSE(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Flush()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			gdriveActive := make([]*GDriveJob, 0)
			h.gdriveJobs.Range(func(key, value interface{}) bool {
				if job, ok := value.(*GDriveJob); ok {
					gdriveActive = append(gdriveActive, job)
				}
				return true
			})

			reencodeActive := make([]interface{}, 0)
			if h.reencodeJobs != nil {
				h.reencodeJobs.Range(func(key, value interface{}) bool {
					if value != nil {
						reencodeActive = append(reencodeActive, value)
					}
					return true
				})
			}

			c.SSEvent("message", gin.H{
				"gdrive":   gdriveActive,
				"reencode": reencodeActive,
			})
			c.Writer.Flush()
		}
	}
}

type CachedFileWithType struct {
	Name       string  `json:"name"`
	Path       string  `json:"path"`
	Size       int64   `json:"size"`
	Type       string  `json:"type"` // magnet or direct
	InfoHash   string  `json:"infoHash,omitempty"`
	FileIndex  int     `json:"fileIndex,omitempty"`
	DownloadID int     `json:"downloadId,omitempty"`
	Progress   float64 `json:"progress,omitempty"`
	Status     string  `json:"status,omitempty"`
	StreamURL  string  `json:"streamUrl"`
	CanPlay    bool    `json:"canPlay"`
}

// isVideoFile checks if the file is a video file
func isVideoFile(name string) bool {
	videoExtensions := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts", ".m2ts"}
	ext := strings.ToLower(filepath.Ext(name))
	for _, ve := range videoExtensions {
		if ext == ve {
			return true
		}
	}
	return false
}

// isPossibleInfoHash checks if a string looks like an infoHash (40 hex characters)
func isPossibleInfoHash(s string) bool {
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

// HandleListCachedFiles handles GET /api/cache
func (h *CacheHandler) HandleListCachedFiles(c *gin.Context) {
	var cachedFiles []CachedFileWithType

	// Build a map of torrent names to infoHashes from active torrents
	torrents := h.torrentService.ListTorrents()
	nameToInfoHash := make(map[string]string)
	infoHashToFiles := make(map[string][]map[string]interface{})
	fileNameToTorrent := make(map[string]map[string]string) // filename -> {infoHash, fileIndex}

	for _, t := range torrents {
		nameToInfoHash[t.Name] = t.InfoHash
		infoHashToFiles[t.InfoHash] = make([]map[string]interface{}, 0)
		for _, f := range t.Files {
			fileInfo := map[string]interface{}{
				"name":  f.Name,
				"index": f.Index,
			}
			infoHashToFiles[t.InfoHash] = append(infoHashToFiles[t.InfoHash], fileInfo)
			// Also build a direct filename lookup for single-file torrents
			fileNameToTorrent[f.Name] = map[string]string{
				"infoHash":  t.InfoHash,
				"fileIndex": fmt.Sprintf("%d", f.Index),
			}
		}
	}

	// Walk through cache directory
	err := filepath.Walk(h.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Skip directories
		if info.IsDir() {
			// Don't treat direct downloads as magnet cache
			if h.directCacheDir != "" && filepath.Clean(path) == filepath.Clean(h.directCacheDir) {
				return filepath.SkipDir
			}
			// Don't treat exports folder as magnet cache
			if filepath.Base(path) == "exports" {
				return filepath.SkipDir
			}
			return nil
		}

		// Only include video files
		if !isVideoFile(info.Name()) {
			return nil
		}

		// Get relative path from cache dir
		relPath, _ := filepath.Rel(h.cacheDir, path)
		// Normalize path separators to forward slashes for consistent splitting
		relPath = filepath.ToSlash(relPath)
		parts := strings.Split(relPath, "/")

		// Expected structures:
		// - <torrent_name>/<filename> (multi-file torrent - common case)
		// - <infoHash>/<filename> (single-file or special case)
		// - <filename> (single-file torrent at root level)
		var infoHash string
		var fileIndex int

		fileName := info.Name()

		if len(parts) >= 2 {
			// Has subfolder structure: <folder>/<filename>
			folderName := parts[0]

			// 1. Check if folder name looks like an infoHash (40 hex chars)
			if isPossibleInfoHash(folderName) {
				infoHash = folderName
				// Try to find file index from active torrents
				if files, ok := infoHashToFiles[infoHash]; ok {
					for _, f := range files {
						if fName, ok := f["name"].(string); ok && fName == fileName {
							if idx, ok := f["index"].(int); ok {
								fileIndex = idx
							}
							break
						}
					}
				}
			} else
			// 2. Try to match folder name against torrent names
			if hash, ok := nameToInfoHash[folderName]; ok {
				infoHash = hash
				// Try to find the file index by matching file name
				if files, ok := infoHashToFiles[hash]; ok {
					for _, f := range files {
						if fName, ok := f["name"].(string); ok && fName == fileName {
							if idx, ok := f["index"].(int); ok {
								fileIndex = idx
							}
							break
						}
					}
				}
			} else {
				// 3. Folder name not found - might be from inactive torrent
				log.Printf("⚠️ Unknown folder in cache: %s (file: %s)", folderName, fileName)
				infoHash = ""
				fileIndex = 0
			}
		} else {
			// File at root level - single-file torrent
			// Try to match by filename against active torrents
			if match, ok := fileNameToTorrent[fileName]; ok {
				infoHash = match["infoHash"]
				if idxStr := match["fileIndex"]; idxStr != "" {
					if idx, err := strconv.Atoi(idxStr); err == nil {
						fileIndex = idx
					}
				}
			} else {
				log.Printf("⚠️ Unknown file at root level: %s", fileName)
				infoHash = ""
				fileIndex = 0
			}
		}

		streamURL := ""
		canPlay := false
		if infoHash != "" {
			streamURL = fmt.Sprintf("/stream/%s/%d", infoHash, fileIndex)
			canPlay = true
		}

		cachedFile := CachedFileWithType{
			Name:      info.Name(),
			Path:      relPath,
			Size:      info.Size(),
			Type:      "magnet",
			InfoHash:  infoHash,
			FileIndex: fileIndex,
			StreamURL: streamURL,
			CanPlay:   canPlay,
		}

		cachedFiles = append(cachedFiles, cachedFile)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan cache directory"})
		return
	}

	// Add exported (reencoded) files
	exportDir := filepath.Join(h.cacheDir, "exports")
	_ = filepath.Walk(exportDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !isVideoFile(info.Name()) {
			return nil
		}

		relPath, _ := filepath.Rel(exportDir, path)
		relPath = filepath.ToSlash(relPath)

		// Create a fake stream URL that just downloads the file
		// We'll add a new endpoint or use an existing one if possible.
		// For now, let's just make it downloadable via a new /exports route or similar.
		// Actually, I should add a route to serve these files.

		cachedFiles = append(cachedFiles, CachedFileWithType{
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

	// Add direct downloads (DB + filesystem)
	if h.directService != nil && h.directCacheDir != "" {
		downloads, err := h.directService.ListDownloads()
		if err == nil {
			byFilePath := make(map[string]struct {
				id       int
				status   string
				progress float64
				path     string
			})
			for _, dl := range downloads {
				if dl.FilePath == "" {
					continue
				}
				byFilePath[filepath.Clean(dl.FilePath)] = struct {
					id       int
					status   string
					progress float64
					path     string
				}{
					id:       dl.ID,
					status:   dl.Status,
					progress: dl.Progress,
					path:     dl.FilePath,
				}
			}

			// Files on disk
			_ = filepath.Walk(h.directCacheDir, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil {
					return nil
				}
				if info.IsDir() {
					return nil
				}
				if !isVideoFile(info.Name()) {
					return nil
				}

				relPath, _ := filepath.Rel(h.directCacheDir, path)
				relPath = filepath.ToSlash(relPath)

				rec, ok := byFilePath[filepath.Clean(path)]
				if !ok {
					cachedFiles = append(cachedFiles, CachedFileWithType{
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

				cachedFiles = append(cachedFiles, CachedFileWithType{
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

			// DB records missing on disk
			for _, dl := range downloads {
				if dl.FilePath == "" {
					continue
				}
				if _, statErr := os.Stat(dl.FilePath); statErr == nil {
					continue
				}

				cachedFiles = append(cachedFiles, CachedFileWithType{
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

	c.JSON(http.StatusOK, cachedFiles)
}

// HandleGDriveUpload handles POST /api/gdrive/upload
func (h *CacheHandler) HandleGDriveUpload(c *gin.Context) {
	if h.gdriveClient == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Google Drive integration not configured"})
		return
	}

	var req struct {
		InfoHash   string `json:"infoHash"`
		FileIndex  int    `json:"fileIndex"`
		DownloadID int    `json:"downloadId"`
		ExportPath string `json:"exportPath"` // For reencoded files
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var filePath string
	var filename string
	var jobID string

	if req.ExportPath != "" {
		// Handle reencoded files (exports)
		// relPath expected like "exports/infoHash/file.mp4"
		cleanPath := filepath.Clean(strings.TrimPrefix(req.ExportPath, "/"))
		filePath = filepath.Join(h.cacheDir, cleanPath)
		filename = filepath.Base(filePath)
		jobID = "export_" + cleanPath
	} else if req.InfoHash != "" {
		// Handle torrent files
		t := h.torrentService.GetTorrent(req.InfoHash)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Torrent session not active. If it was completed, it should be in the library."})
			return
		}

		// Try to find the file index
		torrents := h.torrentService.ListTorrents()
		var foundFile domain.File
		var torrentName string
		for _, torrentStat := range torrents {
			if torrentStat.InfoHash == req.InfoHash {
				torrentName = torrentStat.Name
				for _, f := range torrentStat.Files {
					if f.Index == req.FileIndex {
						foundFile = f
						break
					}
				}
				break
			}
		}

		if foundFile.Name == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found in torrent"})
			return
		}

		filename = foundFile.Name
		jobID = fmt.Sprintf("torrent_%s_%d", req.InfoHash, req.FileIndex)
		
		// Search for the file on disk in cacheDir
		candidates := []string{
			filepath.Join(h.cacheDir, torrentName, filename),
			filepath.Join(h.cacheDir, filename),
			filepath.Join(h.cacheDir, req.InfoHash, filename),
		}

		for _, path := range candidates {
			if _, err := os.Stat(path); err == nil {
				filePath = path
				break
			}
		}

		// Brute force search if not found
		if filePath == "" {
			_ = filepath.Walk(h.cacheDir, func(path string, info os.FileInfo, err error) error {
				if err == nil && !info.IsDir() && info.Name() == filepath.Base(filename) {
					filePath = path
					return filepath.SkipDir // Found it
				}
				return nil
			})
		}
	} else if req.DownloadID != 0 {
		// Handle direct downloads
		downloads, err := h.directService.ListDownloads()
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
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File path could not be resolved",
			"debug": fmt.Sprintf("req: %+v, cacheDir: %s", req, h.cacheDir),
		})
		return
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "File not found on disk",
			"path":  filePath,
		})
		return
	}

	// Create job with cancelable context
	ctx, cancel := context.WithCancel(context.Background())
	job := &GDriveJob{
		ID:       jobID,
		Filename: filename,
		Status:   "uploading",
		cancel:   cancel,
	}
	h.gdriveJobs.Store(jobID, job)

	// Start upload in background
	go func() {
		log.Printf("☁️ [GDrive] Upload started: %s (ID: %s)", filename, jobID)
		
		_, link, err := h.gdriveClient.Upload(ctx, filePath, filename, func(p float64) {
			job.Progress = p
		})

		if err != nil {
			if ctx.Err() == context.Canceled {
				log.Printf("🛑 [GDrive] Upload canceled by user: %s", filename)
				job.Status = "canceled"
			} else {
				log.Printf("❌ [GDrive] Upload failed for %s: %v", filename, err)
				job.Status = "failed"
				job.Error = err.Error()
			}
			time.AfterFunc(10*time.Minute, func() { h.gdriveJobs.Delete(jobID) })
		} else {
			log.Printf("✅ [GDrive] Upload success: %s -> %s", filename, link)
			job.Status = "completed"
			job.Progress = 100
			job.Link = link
			time.AfterFunc(5*time.Minute, func() { h.gdriveJobs.Delete(jobID) })
		}
	}()

	c.JSON(http.StatusOK, gin.H{"message": "Upload started in background", "resolvedPath": filePath})
}

// HandleCancelGDrive handles POST /api/gdrive/cancel
func (h *CacheHandler) HandleCancelGDrive(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if val, ok := h.gdriveJobs.Load(req.ID); ok {
		job := val.(*GDriveJob)
		if job.cancel != nil {
			job.cancel()
			log.Printf("📥 [GDrive] Received cancel request for ID: %s", req.ID)
			c.JSON(http.StatusOK, gin.H{"message": "GDrive upload cancel requested"})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "Job not found or already finished"})
}

// HandleGDriveSSE handles GET /api/gdrive/stream
func (h *CacheHandler) HandleGDriveSSE(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Flush()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			var activeJobs []*GDriveJob
			h.gdriveJobs.Range(func(key, value interface{}) bool {
				activeJobs = append(activeJobs, value.(*GDriveJob))
				return true
			})
			c.SSEvent("message", activeJobs)
			c.Writer.Flush()
		}
	}
}

// HandleDeleteCachedFile handles DELETE /api/cache/:infoHash
func (h *CacheHandler) HandleDeleteCachedFile(c *gin.Context) {
	infoHash := c.Param("infoHash")
	if infoHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Info hash required"})
		return
	}

	// Remove the entire folder for this infohash
	folderPath := filepath.Join(h.cacheDir, infoHash)

	// Check if folder exists
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Cache folder not found"})
		return
	}

	// Remove folder and all contents
	if err := os.RemoveAll(folderPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete cache: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Cache deleted"})
}

// clearDirectory clears all contents of a directory (except the directory itself)
func (h *CacheHandler) clearDirectory(dir string, skipFiles []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Always skip the direct-download cache folder when clearing torrent cache
		if h.directCacheDir != "" && entry.Name() == filepath.Base(h.directCacheDir) {
			continue
		}

		// Check if we should skip this file/folder
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

		path := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			log.Printf("⚠️ Failed to remove %s: %v", path, err)
		}
	}

	return nil
}

// HandleRemoveAllCache handles DELETE /api/cache/all
func (h *CacheHandler) HandleRemoveAllCache(c *gin.Context) {
	// Clear torrent_data cache (skip torrents.db files)
	if err := h.clearDirectory(h.cacheDir, []string{"torrents.db", "torrents.db-journal"}); err != nil {
		log.Printf("⚠️ Failed to clear torrent cache: %v", err)
	}

	// Clear hls_cache (no files to skip)
	if err := h.clearDirectory(h.hlsCacheDir, []string{}); err != nil {
		log.Printf("⚠️ Failed to clear HLS cache: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "All cache cleared (torrent data + HLS cache)"})
}

// HandleCacheStats handles GET /api/cache/stats
func (h *CacheHandler) HandleCacheStats(c *gin.Context) {
	var totalSize int64
	var fileCount int

	filepath.Walk(h.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			totalSize += info.Size()
			fileCount++
		}
		return nil
	})

	c.JSON(http.StatusOK, gin.H{
		"totalSize": totalSize,
		"fileCount": fileCount,
		"cacheDir":  h.cacheDir,
	})
}

// HandleServeExport handles GET /api/exports/*path
func (h *CacheHandler) HandleServeExport(c *gin.Context) {
	relPath := c.Param("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path required"})
		return
	}

	// Clean path to prevent directory traversal
	relPath = filepath.Clean(strings.TrimPrefix(relPath, "/"))
	fullPath := filepath.Join(h.cacheDir, "exports", relPath)

	// Ensure the file exists and is within the exports directory
	if !strings.HasPrefix(fullPath, filepath.Join(h.cacheDir, "exports")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Export file not found"})
		return
	}

	// Set content disposition to download
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(fullPath)))

	// Use gin's built-in File server to support ranges
	c.File(fullPath)
}
