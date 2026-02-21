package handler

import (
	"encoding/base64"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	cpmodel "torrent-stream/internal/model/custom_provider"
	cprepo "torrent-stream/internal/repository/custom_provider"
	scriptExecutorUC "torrent-stream/internal/usecase/script_executor"
	torrentUC "torrent-stream/internal/usecase/torrent"
)

// TorrentHandler handles torrent-related requests
type TorrentHandler struct {
	service        *torrentUC.Service
	scriptExecutor *scriptExecutorUC.Service
	cpRepo         *cprepo.CustomProviderRepository
}

// NewTorrentHandler creates a new torrent handler
func NewTorrentHandler(service *torrentUC.Service, scriptExecutor *scriptExecutorUC.Service, cpRepo *cprepo.CustomProviderRepository) *TorrentHandler {
	return &TorrentHandler{
		service:        service,
		scriptExecutor: scriptExecutor,
		cpRepo:         cpRepo,
	}
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

// HandleRemoveAll handles DELETE /api/torrents/all
func (h *TorrentHandler) HandleRemoveAll(c *gin.Context) {
	err := h.service.RemoveAllTorrents()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All torrents removed"})
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
	providers := h.service.GetHardcodedProviders()

	var result []torrentUC.ProviderInfo
	for _, p := range providers {
		if p == "all" {
			result = append(result, torrentUC.ProviderInfo{ID: "all", Name: "all", Type: "embedded", PageType: "list"})
		} else {
			result = append(result, torrentUC.ProviderInfo{ID: p, Name: p, Type: "embedded", PageType: "list"})
		}
	}

	customProviders, err := h.cpRepo.GetAll()
	if err == nil {
		for _, cp := range customProviders {
			result = append(result, torrentUC.ProviderInfo{
				ID:       cp.ID,
				Name:     cp.Name,
				Type:     "custom",
				PageType: cp.PageTypeDefault,
			})
		}
	}

	c.JSON(http.StatusOK, result)
}

// HandleSearchCustom handles GET /api/search/custom/:id
func (h *TorrentHandler) HandleSearchCustom(c *gin.Context) {
	id := c.Param("id")
	query := c.Query("query")
	detailURL := c.Query("detailUrl")

	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider ID required"})
		return
	}

	cp, err := h.cpRepo.GetByID(id)
	if err != nil || cp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Custom provider not found"})
		return
	}

	var fullURL string
	var pageType string

	if detailURL != "" {
		fullURL = detailURL
		pageType = "detail"
	} else {
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Query required for list search"})
			return
		}
		fullURL = cp.BaseURL
		fullURL = replacePlaceholders(fullURL, query)
		pageType = cp.PageTypeDefault
		if pageType == "" {
			pageType = "list"
		}
	}

	code := cp.Code
	decoded, err := base64.StdEncoding.DecodeString(code)
	if err == nil {
		code = string(decoded)
	}

	language := cp.Language
	if language == "" {
		language = "javascript"
	}

	result, err := h.scriptExecutor.Execute(c.Request.Context(), code, fullURL, pageType, language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func replacePlaceholders(template, query string) string {
	return replaceAll(template, "{q}", query,
		"{query}", query,
		"{search}", query,
		"{raw_q}", query,
		"{raw_query}", query,
		"{raw_search}", query)
}

func replaceAll(s string, pairs ...string) string {
	for i := 0; i < len(pairs); i += 2 {
		if i+1 < len(pairs) {
			s = replaceString(s, pairs[i], pairs[i+1])
		}
	}
	return s
}

func replaceString(s, old, new string) string {
	result := ""
	for i := 0; i < len(s); {
		if i+len(old) <= len(s) && s[i:i+len(old)] == old {
			result += new
			i += len(old)
		} else {
			result += string(s[i])
			i++
		}
	}
	return result
}

// HandleSearchCustomDetail handles GET /api/search/custom/:id/detail
func (h *TorrentHandler) HandleSearchCustomDetail(c *gin.Context) {
	id := c.Param("id")
	detailURL := c.Query("url")

	if id == "" || detailURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Provider ID and URL required"})
		return
	}

	cp, err := h.cpRepo.GetByID(id)
	if err != nil || cp == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Custom provider not found"})
		return
	}

	code := cp.Code
	decoded, err := base64.StdEncoding.DecodeString(code)
	if err == nil {
		code = string(decoded)
	}

	language := cp.Language
	if language == "" {
		language = "javascript"
	}

	result, err := h.scriptExecutor.Execute(c.Request.Context(), code, detailURL, "detail", language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

type CustomProviderResponse struct {
	*cpmodel.CustomProvider
	CodeHidden bool `json:"codeHidden"`
}

// HandleStatsSSE handles GET /api/stats/:infoHash/stream
func (h *TorrentHandler) HandleStatsSSE(c *gin.Context) {
	infoHash := c.Param("infoHash")
	if infoHash == "" {
		c.Status(http.StatusBadRequest)
		return
	}

	// Set headers for SSE
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")

	// Flush the response immediately to ensure the client sees the connection
	c.Writer.Flush()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Initial send
	stats, err := h.service.GetStats(infoHash)
	if err == nil {
		c.SSEvent("message", stats)
		c.Writer.Flush()
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			stats, err := h.service.GetStats(infoHash)
			if err != nil {
				// If torrent is gone, maybe stop? For now just keep trying or send empty?
				// Sending error event could be useful
				continue
			}
			c.SSEvent("message", stats)
			c.Writer.Flush()
		}
	}
}
