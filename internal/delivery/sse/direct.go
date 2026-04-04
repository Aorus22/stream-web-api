package sse

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

func (h *Handler) HandleAllDirectDownloadsSSE(c *gin.Context) {
	setSSEHeaders(c)

	ticker := newTicker()
	defer ticker.Stop()

	dls, _ := h.directService.ListDownloads()
	c.SSEvent("message", dls)
	c.Writer.Flush()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			dls, err := h.directService.ListDownloads()
			if err != nil {
				continue
			}
			c.SSEvent("message", dls)
			c.Writer.Flush()
		}
	}
}

func (h *Handler) HandleDirectDownloadProgress(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.Status(badRequest)
		return
	}

	setSSEHeaders(c)

	ch := h.directService.StreamProgress(c.Request.Context(), id)
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
