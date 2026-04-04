package config

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Port           int
	DataDir        string
	CacheDir       string
	HLSCacheDir    string
	DirectCacheDir string
}

func Load() *Config {
	port := 6432
	if p := os.Getenv("PORT"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			port = parsed
		}
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	cacheDir := os.Getenv("CACHE_DIR")
	if cacheDir == "" {
		cacheDir = "./torrent_data"
	}

	hlsCacheDir := os.Getenv("HLS_CACHE_DIR")
	if hlsCacheDir == "" {
		hlsCacheDir = "./hls_cache"
	}

	directCacheDir := filepath.Join(cacheDir, "direct_downloads")

	cfg := &Config{
		Port:           port,
		DataDir:        dataDir,
		CacheDir:       cacheDir,
		HLSCacheDir:    hlsCacheDir,
		DirectCacheDir: directCacheDir,
	}

	return cfg
}

func (c *Config) EnsureDirs() {
	dirs := []string{c.DataDir, c.CacheDir, c.HLSCacheDir, c.DirectCacheDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
}

type GDriveConfig struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
}

func LoadGDriveConfig() *GDriveConfig {
	return &GDriveConfig{
		ClientID:     os.Getenv("GDRIVE_CLIENT_ID"),
		ClientSecret: os.Getenv("GDRIVE_CLIENT_SECRET"),
		RefreshToken: os.Getenv("GDRIVE_REFRESH_TOKEN"),
	}
}

func (g *GDriveConfig) IsConfigured() bool {
	return g.ClientID != "" && g.ClientSecret != "" && g.RefreshToken != ""
}
