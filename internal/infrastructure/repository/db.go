package repository

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
)

func NewSharedDB(cacheDir string) (*sql.DB, error) {
	dbPath := filepath.Join(cacheDir, "torrents.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS active_torrents (
		info_hash TEXT PRIMARY KEY,
		magnet_uri TEXT NOT NULL,
		added_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS direct_downloads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL,
		filename TEXT NOT NULL,
		status TEXT NOT NULL,
		progress REAL DEFAULT 0,
		downloaded_bytes INTEGER DEFAULT 0,
		total_bytes INTEGER DEFAULT 0,
		added_at DATETIME,
		completed_at DATETIME,
		file_path TEXT
	);

	CREATE TABLE IF NOT EXISTS torrent_metadata (
		info_hash TEXT PRIMARY KEY,
		metadata_json TEXT NOT NULL,
		created_at DATETIME
	);
	`
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	log.Printf("Database initialized at: %s", dbPath)
	return db, nil
}
