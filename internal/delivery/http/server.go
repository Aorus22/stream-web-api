package http

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"

	"stream-web-api/internal/delivery/http/handler"
	"stream-web-api/internal/delivery/sse"
)

type Server struct {
	port                  int
	torrentHandler        *handler.TorrentHandler
	streamHandler         *handler.StreamHandler
	subtitleHandler       *handler.SubtitleHandler
	autosyncHandler       *handler.AutoSyncHandler
	catalogHandler        *handler.CatalogHandler
	cacheHandler          *handler.CacheHandler
	directHandler         *handler.DirectDownloadHandler
	scriptExecutorHandler *handler.ScriptExecutorHandler
	customProviderHandler *handler.CustomProviderHandler
	sseHandler            *sse.Handler
}

func NewServer(
	port int,
	torrentHandler *handler.TorrentHandler,
	streamHandler *handler.StreamHandler,
	subtitleHandler *handler.SubtitleHandler,
	autosyncHandler *handler.AutoSyncHandler,
	catalogHandler *handler.CatalogHandler,
	cacheHandler *handler.CacheHandler,
	directHandler *handler.DirectDownloadHandler,
	scriptExecutorHandler *handler.ScriptExecutorHandler,
	customProviderHandler *handler.CustomProviderHandler,
	sseHandler *sse.Handler,
) *Server {
	return &Server{
		port:                  port,
		torrentHandler:        torrentHandler,
		streamHandler:         streamHandler,
		subtitleHandler:       subtitleHandler,
		autosyncHandler:       autosyncHandler,
		catalogHandler:        catalogHandler,
		cacheHandler:          cacheHandler,
		directHandler:         directHandler,
		scriptExecutorHandler: scriptExecutorHandler,
		customProviderHandler: customProviderHandler,
		sseHandler:            sseHandler,
	}
}

func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	r := SetupRouter(s)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Server starting on http://0.0.0.0%s", addr)

	return r.Run(addr)
}
