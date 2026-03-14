package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"torrent-stream/internal/usecase/direct"
	"torrent-stream/internal/usecase/torrent"

	"github.com/gin-gonic/gin"
)

// CacheHandler handles cache-related requests
type CacheHandler struct {
	cacheDir       string
	directCacheDir string
	hlsCacheDir    string
	torrentService *torrent.Service
	directService  *direct.Service
}

// NewCacheHandler creates a new cache handler
func NewCacheHandler(cacheDir string, directCacheDir string, hlsCacheDir string, torrentService *torrent.Service, directService *direct.Service) *CacheHandler {
	return &CacheHandler{
		cacheDir:       cacheDir,
		directCacheDir: directCacheDir,
		hlsCacheDir:    hlsCacheDir,
		torrentService: torrentService,
		directService:  directService,
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
			Type:      "direct",
			Status:    "completed",
			StreamURL: "/api/exports/" + relPath, // New endpoint needed
			CanPlay:   false,                      // As requested "ga bisa di stream kayak yang normal"
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
