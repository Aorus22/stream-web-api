package handler

import (
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	uc "stream-web-api/internal/domain/usecase"

	"stream-web-api/internal/domain/model"
)

type CacheHandler struct {
	service *uc.CacheUsecase
}

func NewCacheHandler(service *uc.CacheUsecase) *CacheHandler {
	return &CacheHandler{service: service}
}

func (h *CacheHandler) HandleListCachedFiles(c *gin.Context) {
	files, err := h.service.ListCachedFiles()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, files)
}

func (h *CacheHandler) HandleGDriveUpload(c *gin.Context) {
	var req struct {
		InfoHash   string `json:"infoHash"`
		FileIndex  int    `json:"fileIndex"`
		DownloadID int    `json:"downloadId"`
		ExportPath string `json:"exportPath"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	params := &model.GDriveUploadParams{
		InfoHash:   req.InfoHash,
		FileIndex:  req.FileIndex,
		DownloadID: req.DownloadID,
		ExportPath: req.ExportPath,
	}

	resolvedPath, _, err := h.service.StartGDriveUpload(params)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Upload started in background", "resolvedPath": resolvedPath})
}

func (h *CacheHandler) HandleCancelGDrive(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.service.CancelGDriveUpload(req.ID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "GDrive upload cancel requested"})
}

func (h *CacheHandler) HandleDeleteCachedFile(c *gin.Context) {
	infoHash := c.Param("infoHash")
	if infoHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Info hash required"})
		return
	}

	if err := h.service.DeleteCache(infoHash); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Cache deleted"})
}

func (h *CacheHandler) HandleRemoveAllCache(c *gin.Context) {
	if err := h.service.ClearAllCache(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "All cache cleared"})
}

func (h *CacheHandler) HandleCacheStats(c *gin.Context) {
	stats, err := h.service.GetCacheStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

func (h *CacheHandler) HandleServeExport(c *gin.Context) {
	relPath := c.Param("path")
	if relPath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path required"})
		return
	}

	fullPath, err := h.service.ResolveExportPath(relPath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Disposition", filepath.Base(fullPath))
	c.File(fullPath)
}
