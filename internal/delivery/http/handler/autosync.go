package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"torrent-stream/internal/domain"
	autosyncUC "torrent-stream/internal/usecase/autosync"
	subtitleUC "torrent-stream/internal/usecase/subtitle"
)

// AutoSyncHandler handles auto-sync requests
type AutoSyncHandler struct {
	autosyncService *autosyncUC.Service
	subtitleService *subtitleUC.Service
	port            int
}

// NewAutoSyncHandler creates a new autosync handler
func NewAutoSyncHandler(autosyncService *autosyncUC.Service, subtitleService *subtitleUC.Service, port int) *AutoSyncHandler {
	return &AutoSyncHandler{
		autosyncService: autosyncService,
		subtitleService: subtitleService,
		port:            port,
	}
}

// HandleAutoSync handles GET /api/subtitles/autosync
func (h *AutoSyncHandler) HandleAutoSync(c *gin.Context) {
	link := c.Query("link")
	infoHash := c.Query("infoHash")
	fileIndexStr := c.Query("fileIndex")

	if link == "" || infoHash == "" || fileIndexStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing params"})
		return
	}

	fileIndex, _ := strconv.Atoi(fileIndexStr)

	// Parse currentTime
	currentTimeStr := c.Query("currentTime")
	currentTime := 0.0
	if currentTimeStr != "" {
		currentTime, _ = strconv.ParseFloat(currentTimeStr, 64)
	}

	// Download subtitle content
	srtContent, err := h.subtitleService.DownloadRaw(link)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to download subtitle"})
		return
	}

	// Create request
	req := domain.AutoSyncRequest{
		InfoHash:    infoHash,
		FileIndex:   fileIndex,
		SubLink:     link,
		CurrentTime: currentTime,
	}

	// Calculate offset
	result, err := h.autosyncService.CalculateOffset(req, srtContent, h.port)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AutoSync failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
