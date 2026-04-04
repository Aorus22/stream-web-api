package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	uc "stream-web-api/internal/domain/usecase"
)

type DirectDownloadHandler struct {
	service *uc.DirectDownloadUsecase
}

func NewDirectDownloadHandler(service *uc.DirectDownloadUsecase) *DirectDownloadHandler {
	return &DirectDownloadHandler{service: service}
}

func (h *DirectDownloadHandler) HandleAddDirectDownload(c *gin.Context) {
	var urlStr string
	mode := c.Query("mode")

	urlStr = c.PostForm("url")
	if urlStr == "" {
		var body struct {
			URL  string `json:"url"`
			Mode string `json:"mode"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			urlStr = body.URL
			if mode == "" {
				mode = body.Mode
			}
		}
	}

	if urlStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "url required"})
		return
	}

	dl, err := h.service.AddWithMode(urlStr, mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dl)
}

func (h *DirectDownloadHandler) HandleListDirectDownloads(c *gin.Context) {
	dls, err := h.service.ListDownloads()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list downloads"})
		return
	}
	c.JSON(http.StatusOK, dls)
}

func (h *DirectDownloadHandler) HandleGetDirectDownload(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	dl, err := h.service.GetDownload(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "download not found"})
		return
	}

	c.JSON(http.StatusOK, dl)
}

func (h *DirectDownloadHandler) HandleDeleteDirectDownload(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.service.DeleteDownload(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "download not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

func (h *DirectDownloadHandler) HandleDirectDownloadAll(c *gin.Context) {
	if err := h.service.DeleteAll(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete downloads"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
