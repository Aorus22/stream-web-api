package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	torrentUC "torrent-stream/internal/usecase/torrent"
)

// TorrentHandler handles torrent-related requests
type TorrentHandler struct {
	service *torrentUC.Service
}

// NewTorrentHandler creates a new torrent handler
func NewTorrentHandler(service *torrentUC.Service) *TorrentHandler {
	return &TorrentHandler{service: service}
}

// HandleAddMagnet handles POST /api/add
func (h *TorrentHandler) HandleAddMagnet(c *gin.Context) {
	magnet := c.PostForm("magnet")
	if magnet == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Magnet link required"})
		return
	}

	stats, err := h.service.AddMagnet(magnet)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// HandleListTorrents handles GET /api/torrents
func (h *TorrentHandler) HandleListTorrents(c *gin.Context) {
	stats := h.service.ListTorrents()
	c.JSON(http.StatusOK, stats)
}

// HandleStats handles GET /api/stats/:infoHash
func (h *TorrentHandler) HandleStats(c *gin.Context) {
	infoHash := c.Param("infoHash")
	if infoHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Info hash required"})
		return
	}

	stats, err := h.service.GetStats(infoHash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// HandlePieceInfo handles GET /api/pieces/:infoHash/:fileIndex
func (h *TorrentHandler) HandlePieceInfo(c *gin.Context) {
	infoHash := c.Param("infoHash")
	fileIndex, err := strconv.Atoi(c.Param("fileIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file index"})
		return
	}

	info, err := h.service.GetPieceInfo(infoHash, fileIndex)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// HandleRemove handles DELETE /api/remove/:infoHash
func (h *TorrentHandler) HandleRemove(c *gin.Context) {
	infoHash := c.Param("infoHash")
	if infoHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Info hash required"})
		return
	}

	err := h.service.RemoveTorrent(infoHash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "OK"})
}

// HandleSearch handles GET /api/search
func (h *TorrentHandler) HandleSearch(c *gin.Context) {
	provider := c.Query("provider")
	query := c.Query("query")
	pageStr := c.DefaultQuery("page", "1")

	if provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider required"})
		return
	}
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query required"})
		return
	}

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	results, err := h.service.SearchTorrents(provider, query, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// HandleListProviders handles GET /api/providers
func (h *TorrentHandler) HandleListProviders(c *gin.Context) {
	providers := h.service.GetSearchProviders()
	c.JSON(http.StatusOK, providers)
}
