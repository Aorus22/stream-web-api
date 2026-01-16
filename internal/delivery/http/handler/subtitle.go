package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	subtitleUC "torrent-stream/internal/usecase/subtitle"
)

// SubtitleHandler handles subtitle-related requests
type SubtitleHandler struct {
	service *subtitleUC.Service
}

// NewSubtitleHandler creates a new subtitle handler
func NewSubtitleHandler(service *subtitleUC.Service) *SubtitleHandler {
	return &SubtitleHandler{service: service}
}

// HandleSearch handles GET /api/subtitles/search
func (h *SubtitleHandler) HandleSearch(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query required"})
		return
	}

	lang := c.Query("lang")

	subtitles, err := h.service.Search(query, lang)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, subtitles)
}

// HandleDownload handles GET /api/subtitles/download
func (h *SubtitleHandler) HandleDownload(c *gin.Context) {
	link := c.Query("link")
	if link == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Link required"})
		return
	}

	cues, err := h.service.Download(link)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cues)
}
