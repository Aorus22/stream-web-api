package handler

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// CacheHandler handles cache-related requests
type CacheHandler struct {
	cacheDir    string
	hlsCacheDir string
}

// NewCacheHandler creates a new cache handler
func NewCacheHandler(cacheDir string, hlsCacheDir string) *CacheHandler {
	return &CacheHandler{
		cacheDir:    cacheDir,
		hlsCacheDir: hlsCacheDir,
	}
}

// CachedFile represents a cached file
type CachedFile struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	Size      int64  `json:"size"`
	InfoHash  string `json:"infoHash"`
	FileIndex int    `json:"fileIndex"`
	StreamURL string `json:"streamUrl"`
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

// HandleListCachedFiles handles GET /api/cache
func (h *CacheHandler) HandleListCachedFiles(c *gin.Context) {
	var cachedFiles []CachedFile

	// Walk through cache directory
	err := filepath.Walk(h.cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only include video files
		if !isVideoFile(info.Name()) {
			return nil
		}

		// Get relative path from cache dir
		relPath, _ := filepath.Rel(h.cacheDir, path)
		parts := strings.Split(relPath, string(os.PathSeparator))

		// Expected structure: <infoHash>/<filename>
		// The infohash folder contains the downloaded files
		infoHash := ""
		if len(parts) >= 1 {
			infoHash = parts[0]
		}

		cachedFile := CachedFile{
			Name:      info.Name(),
			Path:      relPath,
			Size:      info.Size(),
			InfoHash:  infoHash,
			FileIndex: 0, // Default, would need torrent metadata to get actual index
		}

		cachedFiles = append(cachedFiles, cachedFile)
		return nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan cache directory"})
		return
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
