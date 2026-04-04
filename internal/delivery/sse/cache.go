package sse

import "github.com/gin-gonic/gin"

func (h *Handler) HandleTasksSSE(c *gin.Context) {
	setSSEHeaders(c)

	ticker := newTicker()
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			event := h.cacheService.GetTasksEvent()
			c.SSEvent("message", event)
			c.Writer.Flush()
		}
	}
}
