package main

import (
	"log"
	"os"

	"torrent-stream/internal/delivery/http"
	"torrent-stream/internal/delivery/http/handler"
	"torrent-stream/internal/infrastructure/ffmpeg"
	"torrent-stream/internal/infrastructure/opensubtitles"
	"torrent-stream/internal/infrastructure/torrent"
	autosyncUC "torrent-stream/internal/usecase/autosync"
	subtitleUC "torrent-stream/internal/usecase/subtitle"
	torrentUC "torrent-stream/internal/usecase/torrent"
)

func main() {
	port := 6432
	cacheDir := "/tmp/torrent-stream-cache"

	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Fatalf("Failed to create cache directory: %v", err)
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

	// 2. Use Cases
	torrentService := torrentUC.NewService(torrentClient, port)
	subtitleService := subtitleUC.NewService(opensubtitlesClient)
	autosyncService := autosyncUC.NewService(transcoder)

	// 3. Handlers
	torrentHandler := handler.NewTorrentHandler(torrentService)
	streamHandler := handler.NewStreamHandler(torrentService, transcoder)
	subtitleHandler := handler.NewSubtitleHandler(subtitleService)
	autosyncHandler := handler.NewAutoSyncHandler(autosyncService, subtitleService, port)

	// 4. Server
	server := http.NewServer(
		port,
		torrentHandler,
		streamHandler,
		subtitleHandler,
		autosyncHandler,
	)

	// Start Server
	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
