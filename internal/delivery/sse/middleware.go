package sse

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const badRequest = http.StatusBadRequest

func newTicker() *time.Ticker {
	return time.NewTicker(1 * time.Second)
}

func setSSEHeaders(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("Transfer-Encoding", "chunked")
	c.Writer.Flush()
}
