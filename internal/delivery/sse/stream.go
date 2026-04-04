package sse

import "github.com/gin-gonic/gin"

func (h *Handler) HandleReencodeSSE(c *gin.Context) {
	setSSEHeaders(c)

	ticker := newTicker()
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			jobs := h.streamService.GetReencodeJobs()
			c.SSEvent("message", jobs)
			c.Writer.Flush()
		}
	}
}
