package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	uc "stream-web-api/internal/domain/usecase"
)

type TorrentHandler struct {
	service *uc.TorrentUsecase
}

func NewTorrentHandler(service *uc.TorrentUsecase) *TorrentHandler {
	return &TorrentHandler{service: service}
}

func (h *TorrentHandler) HandleAddMagnet(c *gin.Context) {
	magnet := c.PostForm("magnet")
	if magnet == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Magnet link required"})
		return
	}

	metadata := c.PostForm("metadata")

	stats, err := h.service.AddMagnetWithMetadata(magnet, metadata)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func (h *TorrentHandler) HandleListTorrents(c *gin.Context) {
	stats := h.service.ListTorrents()
	c.JSON(http.StatusOK, stats)
}

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

func (h *TorrentHandler) HandleRemoveAll(c *gin.Context) {
	err := h.service.RemoveAllTorrents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All torrents removed"})
}

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

	page, _ := strconv.Atoi(pageStr)

	results, err := h.service.SearchWithDefaults(provider, query, page)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

func (h *TorrentHandler) HandleListProviders(c *gin.Context) {
	result := h.service.ListAllProviders()
	c.JSON(http.StatusOK, result)
}

func (h *TorrentHandler) HandleSearchCustom(c *gin.Context) {
	id := c.Param("id")
	query := c.Query("query")
	detailURL := c.Query("detailUrl")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider ID required"})
		return
	}

	result, err := h.service.SearchCustom(c.Request.Context(), id, query, detailURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *TorrentHandler) HandleSearchCustomDetail(c *gin.Context) {
	id := c.Param("id")
	detailURL := c.Query("url")

	if id == "" || detailURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider ID and URL required"})
		return
	}

	result, err := h.service.SearchCustom(c.Request.Context(), id, "", detailURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *TorrentHandler) HandleGetMetadata(c *gin.Context) {
	infoHash := c.Param("infoHash")
	if infoHash == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Info hash required"})
		return
	}

	jsonStr, err := h.service.GetMetadata(infoHash)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if jsonStr == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "No metadata found"})
		return
	}

	c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(jsonStr))
}
