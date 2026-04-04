package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	uc "stream-web-api/internal/domain/usecase"
)

type SubtitleHandler struct {
	service *uc.SubtitleUsecase
}

func NewSubtitleHandler(service *uc.SubtitleUsecase) *SubtitleHandler {
	return &SubtitleHandler{service: service}
}

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
