package main

import (
	"log"
	"os"
	"path/filepath"

	"torrent-stream/internal/delivery/http"
	"torrent-stream/internal/delivery/http/handler"
	"torrent-stream/internal/infrastructure/cinemeta"
	"torrent-stream/internal/infrastructure/ffmpeg"
	"torrent-stream/internal/infrastructure/opensubtitles"
	"torrent-stream/internal/infrastructure/persistence"
	"torrent-stream/internal/infrastructure/torrent"
	autosyncUC "torrent-stream/internal/usecase/autosync"
	directUC "torrent-stream/internal/usecase/direct"
	jsExecutorUC "torrent-stream/internal/usecase/js_executor"
	subtitleUC "torrent-stream/internal/usecase/subtitle"
	torrentUC "torrent-stream/internal/usecase/torrent"
)

func main() {
	port := 6432
	cacheDir := "./torrent_data"
	hlsCacheDir := "./hls_cache"
	directCacheDir := filepath.Join(cacheDir, "direct_downloads")

	// Ensure cache directories exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Fatalf("Failed to create cache directory: %v", err)
	}
	if err := os.MkdirAll(hlsCacheDir, 0755); err != nil {
		log.Fatalf("Failed to create hls cache directory: %v", err)
	}
	if err := os.MkdirAll(directCacheDir, 0755); err != nil {
		log.Fatalf("Failed to create direct cache directory: %v", err)
	}

	// 1. Infrastructure
	torrentClient, err := torrent.NewClient(cacheDir)
	if err != nil {
		log.Fatalf("Failed to init torrent client: %v", err)
	}
	defer torrentClient.Close()

	transcoder := ffmpeg.NewTranscoder()
	if transcoder == nil {
		log.Println("Warning: Transcoder initialization failed (FFmpeg not found?)")
	}

	opensubtitlesClient := opensubtitles.NewClient()
	cinemetaClient := cinemeta.NewClient()

	// 2. Use Cases
	torrentService := torrentUC.NewService(torrentClient, port)
	subtitleService := subtitleUC.NewService(opensubtitlesClient)
	autosyncService := autosyncUC.NewService(transcoder)
	jsExecutorService := jsExecutorUC.NewService()
	directRepo, err := persistence.NewDirectDownloadRepository(cacheDir)
	if err != nil {
		log.Fatalf("Failed to init direct download persistence: %v", err)
	}
	defer directRepo.Close()

	directService, err := directUC.NewService(directRepo, directCacheDir)
	if err != nil {
		log.Fatalf("Failed to init direct download service: %v", err)
	}

	// 3. Handlers
	torrentHandler := handler.NewTorrentHandler(torrentService)
	streamHandler := handler.NewStreamHandler(torrentService, directService, transcoder, hlsCacheDir)
	subtitleHandler := handler.NewSubtitleHandler(subtitleService)
	autosyncHandler := handler.NewAutoSyncHandler(autosyncService, subtitleService, port)
	catalogHandler := handler.NewCatalogHandler(cinemetaClient)
	cacheHandler := handler.NewCacheHandler(cacheDir, directCacheDir, hlsCacheDir, torrentService, directService)
	directHandler := handler.NewDirectDownloadHandler(directService)
	jsExecutorHandler := handler.NewJSExecutorHandler(jsExecutorService)

	// 4. Server
	server := http.NewServer(
		port,
		torrentHandler,
		streamHandler,
		subtitleHandler,
		autosyncHandler,
		catalogHandler,
		cacheHandler,
		directHandler,
		jsExecutorHandler,
	)

	// Start Server
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
