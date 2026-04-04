package main

import (
	"log"

	"stream-web-api/internal/config"
	"stream-web-api/internal/delivery/http"
	"stream-web-api/internal/delivery/http/handler"
	"stream-web-api/internal/domain/usecase"
	infra "stream-web-api/internal/infrastructure/repository"
)

func main() {
	cfg := config.Load()
	cfg.EnsureDirs()

	gdriveCfg := config.LoadGDriveConfig()

	sharedDB, err := infra.NewSharedDB(cfg.CacheDir)
	if err != nil {
		log.Fatalf("Failed to init shared database: %v", err)
	}
	defer sharedDB.Close()

	torrentClient, err := infra.NewClient(cfg.CacheDir)
	if err != nil {
		log.Fatalf("Failed to init torrent client: %v", err)
	}
	defer torrentClient.Close()

	var gdriveClient *infra.GDriveClient
	if gdriveCfg.IsConfigured() {
		gdriveClient = infra.NewGDriveClient(gdriveCfg.ClientID, gdriveCfg.ClientSecret, gdriveCfg.RefreshToken)
		log.Println("Google Drive integration enabled")
	}

	transcoder := infra.NewTranscoder()
	if transcoder == nil {
		log.Println("Warning: Transcoder initialization failed (FFmpeg not found?)")
	}

	opensubtitlesClient := infra.NewOpenSubtitlesClient()
	cinemetaClient := infra.NewCinemetaClient()
	subtitleDownloader := infra.NewSubtitleDownloader()
	luaExecutor := infra.NewLuaExecutor()

	gormDB, err := infra.NewDB(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to init GORM database: %v", err)
	}

	directRepo := infra.NewDirectDownloadRepository(sharedDB)
	customProviderRepo := infra.NewCustomProviderRepository(gormDB)

	scriptExecutorService := usecase.NewScriptExecutorUsecase(luaExecutor)

	torrentService := usecase.NewTorrentUsecase(torrentClient, cfg.Port)
	torrentService.SetCustomProviderRepo(customProviderRepo)
	torrentService.SetScriptExecutor(scriptExecutorService)

	subtitleService := usecase.NewSubtitleUsecase(opensubtitlesClient, subtitleDownloader)
	autosyncService := usecase.NewAutoSyncUsecase(transcoder)

	directService, err := usecase.NewDirectDownloadUsecase(directRepo, cfg.DirectCacheDir)
	if err != nil {
		log.Fatalf("Failed to init direct download service: %v", err)
	}

	customProviderService := usecase.NewCustomProviderUsecase(customProviderRepo)
	catalogService := usecase.NewCatalogUsecase(cinemetaClient)

	streamService := usecase.NewStreamUsecase(torrentService, directService, transcoder, cfg.CacheDir, cfg.Port)

	cacheService := usecase.NewCacheUsecase(cfg.CacheDir, cfg.DirectCacheDir, cfg.HLSCacheDir, torrentService, directService, gdriveClient)
	cacheService.SetStreamService(streamService)

	torrentHandler := handler.NewTorrentHandler(torrentService)
	streamHandler := handler.NewStreamHandler(streamService, torrentService, directService, cfg.CacheDir)
	subtitleHandler := handler.NewSubtitleHandler(subtitleService)
	autosyncHandler := handler.NewAutoSyncHandler(autosyncService, subtitleService, cfg.Port)
	catalogHandler := handler.NewCatalogHandler(catalogService)
	cacheHandler := handler.NewCacheHandler(cacheService)
	directHandler := handler.NewDirectDownloadHandler(directService)
	scriptExecutorHandler := handler.NewScriptExecutorHandler(scriptExecutorService)
	customProviderHandler := handler.NewCustomProviderHandler(customProviderService)

	server := http.NewServer(
		cfg.Port,
		torrentHandler,
		streamHandler,
		subtitleHandler,
		autosyncHandler,
		catalogHandler,
		cacheHandler,
		directHandler,
		scriptExecutorHandler,
		customProviderHandler,
	)

	if err := server.Start(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
