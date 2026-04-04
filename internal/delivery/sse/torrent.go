package sse

import (
	"github.com/gin-gonic/gin"

	uc "stream-web-api/internal/domain/usecase"
)

type Handler struct {
	torrentService *uc.TorrentUsecase
	directService  *uc.DirectDownloadUsecase
	streamService  *uc.StreamUsecase
	cacheService   *uc.CacheUsecase
}

func NewHandler(
	torrentService *uc.TorrentUsecase,
	directService *uc.DirectDownloadUsecase,
	streamService *uc.StreamUsecase,
	cacheService *uc.CacheUsecase,
) *Handler {
	return &Handler{
		torrentService: torrentService,
		directService:  directService,
		streamService:  streamService,
		cacheService:   cacheService,
	}
}

func (h *Handler) HandleAllTorrentsSSE(c *gin.Context) {
	setSSEHeaders(c)

	ticker := newTicker()
	defer ticker.Stop()

	stats := h.torrentService.ListTorrents()
	c.SSEvent("message", stats)
	c.Writer.Flush()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			stats := h.torrentService.ListTorrents()
			c.SSEvent("message", stats)
			c.Writer.Flush()
		}
	}
}

func (h *Handler) HandleStatsSSE(c *gin.Context) {
	infoHash := c.Param("infoHash")
	if infoHash == "" {
		c.Status(badRequest)
		return
	}

	setSSEHeaders(c)

	ticker := newTicker()
	defer ticker.Stop()

	stats, err := h.torrentService.GetStats(infoHash)
	if err == nil {
		c.SSEvent("message", stats)
		c.Writer.Flush()
	}

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			stats, err := h.torrentService.GetStats(infoHash)
			if err != nil {
				continue
			}
			c.SSEvent("message", stats)
			c.Writer.Flush()
		}
	}
}
