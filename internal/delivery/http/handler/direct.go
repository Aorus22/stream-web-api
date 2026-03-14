package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"torrent-stream/internal/usecase/direct"
)

type DirectDownloadHandler struct {
	service *direct.Service
}

func NewDirectDownloadHandler(service *direct.Service) *DirectDownloadHandler {
	return &DirectDownloadHandler{service: service}
}

func (h *DirectDownloadHandler) HandleAddDirectDownload(c *gin.Context) {
	var urlStr string
	mode := c.Query("mode")

	// Support form-encoded and JSON
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

	var (
		dl  interface{}
		err error
	)
	if mode == "ondemand" || mode == "on_demand" || mode == "stream" {
		dl, err = h.service.AddOnDemand(urlStr)
	} else {
		dl, err = h.service.AddDownload(urlStr)
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, dl)
}

func (h *DirectDownloadHandler) HandleAllDirectDownloadsSSE(c *gin.Context) {
	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Flush()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Initial send
	dls, _ := h.service.ListDownloads()
	c.SSEvent("message", dls)
	c.Writer.Flush()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			dls, err := h.service.ListDownloads()
			if err != nil {
				continue
			}
			c.SSEvent("message", dls)
			c.Writer.Flush()
		}
	}
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

func (h *DirectDownloadHandler) HandleDirectDownloadProgress(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Flush()

	ch := h.service.StreamProgress(c.Request.Context(), id)
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case p, ok := <-ch:
			if !ok {
				return
			}
			c.SSEvent("message", p)
			c.Writer.Flush()
		}
	}
}

func (h *DirectDownloadHandler) HandleDirectDownloadAll(c *gin.Context) {
	if err := h.service.DeleteAll(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete downloads"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
