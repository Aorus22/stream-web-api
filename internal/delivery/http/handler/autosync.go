package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"stream-web-api/internal/domain/model"
	uc "stream-web-api/internal/domain/usecase"
)

type AutoSyncHandler struct {
	autosyncService *uc.AutoSyncUsecase
	subtitleService *uc.SubtitleUsecase
	port            int
}

func NewAutoSyncHandler(autosyncService *uc.AutoSyncUsecase, subtitleService *uc.SubtitleUsecase, port int) *AutoSyncHandler {
	return &AutoSyncHandler{
		autosyncService: autosyncService,
		subtitleService: subtitleService,
		port:            port,
	}
}

func (h *AutoSyncHandler) HandleAutoSync(c *gin.Context) {
	link := c.Query("link")
	infoHash := c.Query("infoHash")
	fileIndexStr := c.Query("fileIndex")

	if link == "" || infoHash == "" || fileIndexStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing params"})
		return
	}

	fileIndex, _ := strconv.Atoi(fileIndexStr)

	currentTimeStr := c.Query("currentTime")
	currentTime := 0.0
	if currentTimeStr != "" {
		currentTime, _ = strconv.ParseFloat(currentTimeStr, 64)
	}

	srtContent, err := h.subtitleService.DownloadRaw(link)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to download subtitle"})
		return
	}

	req := model.AutoSyncRequest{
		InfoHash:    infoHash,
		FileIndex:   fileIndex,
		SubLink:     link,
		CurrentTime: currentTime,
	}

	result, err := h.autosyncService.CalculateOffset(req, srtContent, h.port)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "AutoSync failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
