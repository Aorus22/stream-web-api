package http

import (
	"fmt"
	"log"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"torrent-stream/internal/delivery/http/handler"
)

// Server represents the HTTP server
type Server struct {
	port            int
	torrentHandler  *handler.TorrentHandler
	streamHandler   *handler.StreamHandler
	subtitleHandler *handler.SubtitleHandler
	autosyncHandler *handler.AutoSyncHandler
}

// NewServer creates a new HTTP server
func NewServer(
	port int,
	torrentHandler *handler.TorrentHandler,
	streamHandler *handler.StreamHandler,
	subtitleHandler *handler.SubtitleHandler,
	autosyncHandler *handler.AutoSyncHandler,
) *Server {
	return &Server{
		port:            port,
		torrentHandler:  torrentHandler,
		streamHandler:   streamHandler,
		subtitleHandler: subtitleHandler,
		autosyncHandler: autosyncHandler,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// CORS middleware - Allow ALL
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"*"},
		ExposeHeaders:    []string{"*"},
		AllowCredentials: false,
		MaxAge:           24 * time.Hour,
	}))

	// Torrent routes
	r.POST("/api/add", s.torrentHandler.HandleAddMagnet)
	r.GET("/api/torrents", s.torrentHandler.HandleListTorrents)
	r.GET("/api/stats/:infoHash", s.torrentHandler.HandleStats)
	r.GET("/api/pieces/:infoHash/:fileIndex", s.torrentHandler.HandlePieceInfo)
	r.DELETE("/api/remove/:infoHash", s.torrentHandler.HandleRemove)

	// Stream routes
	r.GET("/stream/:infoHash/:fileIndex", s.streamHandler.HandleStream)
	r.GET("/transcode/:infoHash/:fileIndex", s.streamHandler.HandleTranscode)
	r.GET("/api/duration/:infoHash/:fileIndex", s.streamHandler.HandleDuration)
	r.DELETE("/api/stream/active", s.streamHandler.HandleKillStream)

	// Subtitle routes
	r.GET("/api/subtitles/search", s.subtitleHandler.HandleSearch)
	r.GET("/api/subtitles/download", s.subtitleHandler.HandleDownload)
	r.GET("/api/subtitles/autosync", s.autosyncHandler.HandleAutoSync)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🚀 Server starting on http://0.0.0.0%s", addr)

	return r.Run(addr)
}
