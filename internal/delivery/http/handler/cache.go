package handler

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"torrent-stream/internal/usecase/torrent"
)

// CacheHandler handles cache-related requests
type CacheHandler struct {
	cacheDir       string
	hlsCacheDir    string
	torrentService *torrent.Service
}

// NewCacheHandler creates a new cache handler
func NewCacheHandler(cacheDir string, hlsCacheDir string, torrentService *torrent.Service) *CacheHandler {
	return &CacheHandler{
		cacheDir:       cacheDir,
		hlsCacheDir:    hlsCacheDir,
		torrentService: torrentService,
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
	var cachedFiles []CachedFile

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
				"infoHash":   t.InfoHash,
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

		cachedFile := CachedFile{
			Name:      info.Name(),
			Path:      relPath,
			Size:      info.Size(),
			InfoHash:  infoHash,
			FileIndex: fileIndex,
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
