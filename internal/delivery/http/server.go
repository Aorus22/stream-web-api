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
	directHandler *handler.DirectDownloadHandler,
	scriptExecutorHandler *handler.ScriptExecutorHandler,
	customProviderHandler *handler.CustomProviderHandler,
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
	r.GET("/api/search/custom/:id", s.torrentHandler.HandleSearchCustom)
	r.GET("/api/providers", s.torrentHandler.HandleListProviders)
	r.GET("/api/stats/:infoHash", s.torrentHandler.HandleStats)
	r.GET("/api/pieces/:infoHash/:fileIndex", s.torrentHandler.HandlePieceInfo)
	r.DELETE("/api/remove/:infoHash", s.torrentHandler.HandleRemove)
	r.DELETE("/api/torrents/all", s.torrentHandler.HandleRemoveAll)
	r.GET("/api/torrents/stream", s.torrentHandler.HandleAllTorrentsSSE)
	r.GET("/api/stats/:infoHash/stream", s.torrentHandler.HandleStatsSSE)

	// Stream routes
	r.GET("/stream/:infoHash/:fileIndex", s.streamHandler.HandleStream)
	r.GET("/transcode/:infoHash/:fileIndex", s.streamHandler.HandleTranscode)
	r.POST("/api/reencode", s.streamHandler.HandleReencode)
	r.GET("/api/duration/:infoHash/:fileIndex", s.streamHandler.HandleDuration)
	r.GET("/api/metadata/:infoHash/:fileIndex", s.streamHandler.HandleMediaInfo)
	r.GET("/api/stream/:infoHash/:fileIndex/sub/:streamIndex", s.streamHandler.HandleStreamSubtitle)
	r.DELETE("/api/stream/active", s.streamHandler.HandleKillStream)

	// HLS routes
	r.GET("/hls/:infoHash/:fileIndex/playlist.m3u8", s.streamHandler.HandleHLSMasterPlaylist)
	r.GET("/hls/:infoHash/:fileIndex/segment/:segment", s.streamHandler.HandleHLSSegment)

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
	r.DELETE("/api/cache/all", s.cacheHandler.HandleRemoveAllCache)
	r.DELETE("/api/cache/:infoHash", s.cacheHandler.HandleDeleteCachedFile)
	r.GET("/api/exports/*path", s.cacheHandler.HandleServeExport)

	// Direct download routes
	r.POST("/api/direct/add", s.directHandler.HandleAddDirectDownload)
	r.GET("/api/direct", s.directHandler.HandleListDirectDownloads)
	r.GET("/api/direct/stream", s.directHandler.HandleAllDirectDownloadsSSE)
	r.GET("/api/direct/:id", s.directHandler.HandleGetDirectDownload)
	r.DELETE("/api/direct/:id", s.directHandler.HandleDeleteDirectDownload)
	r.GET("/api/direct/:id/progress", s.directHandler.HandleDirectDownloadProgress)
	r.DELETE("/api/direct/all", s.directHandler.HandleDirectDownloadAll)

	// Direct stream route
	r.GET("/stream/direct/:id", s.streamHandler.HandleDirectStream)

	// Script executor routes
	r.POST("/api/js/execute", s.scriptExecutorHandler.HandleExecuteScript)
	r.POST("/api/js/preview", s.scriptExecutorHandler.HandlePreviewHTML)

	// Custom provider routes
	r.GET("/api/custom-providers", s.customProviderHandler.HandleGetAll)
	r.POST("/api/custom-providers", s.customProviderHandler.HandleCreate)
	r.GET("/api/custom-providers/:id", s.customProviderHandler.HandleGetByID)
	r.PUT("/api/custom-providers/:id", s.customProviderHandler.HandleUpdate)
	r.DELETE("/api/custom-providers/:id", s.customProviderHandler.HandleDelete)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("🚀 Server starting on http://0.0.0.0%s", addr)

	return r.Run(addr)
}
