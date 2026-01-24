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
	catalogHandler  *handler.CatalogHandler
	cacheHandler    *handler.CacheHandler
}

// NewServer creates a new HTTP server
func NewServer(
	port int,
	torrentHandler *handler.TorrentHandler,
	streamHandler *handler.StreamHandler,
	subtitleHandler *handler.SubtitleHandler,
	autosyncHandler *handler.AutoSyncHandler,
	catalogHandler *handler.CatalogHandler,
	cacheHandler *handler.CacheHandler,
) *Server {
	return &Server{
		port:            port,
		torrentHandler:  torrentHandler,
		streamHandler:   streamHandler,
		subtitleHandler: subtitleHandler,
		autosyncHandler: autosyncHandler,
		catalogHandler:  catalogHandler,
		cacheHandler:    cacheHandler,
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
	r.GET("/api/search", s.torrentHandler.HandleSearch)
	r.GET("/api/providers", s.torrentHandler.HandleListProviders)
	r.GET("/api/stats/:infoHash", s.torrentHandler.HandleStats)
	r.GET("/api/pieces/:infoHash/:fileIndex", s.torrentHandler.HandlePieceInfo)
	r.DELETE("/api/remove/:infoHash", s.torrentHandler.HandleRemove)

	// Stream routes
	r.GET("/stream/:infoHash/:fileIndex", s.streamHandler.HandleStream)
	r.GET("/transcode/:infoHash/:fileIndex", s.streamHandler.HandleTranscode)
	r.GET("/api/duration/:infoHash/:fileIndex", s.streamHandler.HandleDuration)
	r.GET("/api/metadata/:infoHash/:fileIndex", s.streamHandler.HandleMediaInfo)
	r.GET("/api/stream/:infoHash/:fileIndex/sub/:streamIndex", s.streamHandler.HandleStreamSubtitle)
	r.DELETE("/api/stream/active", s.streamHandler.HandleKillStream)

	// Subtitle routes
	r.GET("/api/subtitles/search", s.subtitleHandler.HandleSearch)
	r.GET("/api/subtitles/download", s.subtitleHandler.HandleDownload)
	r.GET("/api/subtitles/autosync", s.autosyncHandler.HandleAutoSync)

	// Catalog routes (Cinemeta - Stremio's public API)
	r.GET("/api/catalog/movies", s.catalogHandler.HandleTopMovies)
	r.GET("/api/catalog/series", s.catalogHandler.HandleTopSeries)
	r.GET("/api/catalog/movies/top-rated", s.catalogHandler.HandleTopRatedMovies)
	r.GET("/api/catalog/series/top-rated", s.catalogHandler.HandleTopRatedSeries)
	r.GET("/api/catalog/movies/genre/:genre", s.catalogHandler.HandleGenreMovies)
	r.GET("/api/catalog/series/genre/:genre", s.catalogHandler.HandleGenreSeries)
	r.GET("/api/catalog/movies/search", s.catalogHandler.HandleSearchMovies)
	r.GET("/api/catalog/series/search", s.catalogHandler.HandleSearchSeries)
	r.GET("/api/catalog/movie/:id", s.catalogHandler.HandleMovieDetail)
	r.GET("/api/catalog/series/:id", s.catalogHandler.HandleSeriesDetail)

	// Cache routes
	r.GET("/api/cache", s.cacheHandler.HandleListCachedFiles)
	r.GET("/api/cache/stats", s.cacheHandler.HandleCacheStats)
	r.DELETE("/api/cache/:infoHash", s.cacheHandler.HandleDeleteCachedFile)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🚀 Server starting on http://0.0.0.0%s", addr)

	return r.Run(addr)
}
