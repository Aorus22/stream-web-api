package persistence

import (
	"database/sql"
	"log"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// TorrentRepository handles active torrent persistence
type TorrentRepository struct {
	db *sql.DB
}

// ActiveTorrent represents a persisted active torrent
type ActiveTorrent struct {
	InfoHash  string
	MagnetURI string
	AddedAt   time.Time
}

// NewTorrentRepository creates a new SQLite repository
func NewTorrentRepository(cacheDir string) (*TorrentRepository, error) {
	dbPath := filepath.Join(cacheDir, "torrents.db")

	// Use modernc.org/sqlite driver
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if not exists
	query := `
	CREATE TABLE IF NOT EXISTS active_torrents (
		info_hash TEXT PRIMARY KEY,
		magnet_uri TEXT NOT NULL,
		added_at DATETIME
	);
	`
	if _, err := db.Exec(query); err != nil {
		return nil, err
	}

	return &TorrentRepository{db: db}, nil
}

// Add saves a torrent to the database
func (r *TorrentRepository) Add(infoHash, magnetURI string) error {
	query := `
	INSERT OR REPLACE INTO active_torrents (info_hash, magnet_uri, added_at)
	VALUES (?, ?, ?)
	`
	_, err := r.db.Exec(query, infoHash, magnetURI, time.Now())
	return err
}

// Remove deletes a torrent from the database
func (r *TorrentRepository) Remove(infoHash string) error {
	query := `DELETE FROM active_torrents WHERE info_hash = ?`
	_, err := r.db.Exec(query, infoHash)
	return err
}

// RemoveAll deletes all torrents from the database
func (r *TorrentRepository) RemoveAll() error {
	query := `DELETE FROM active_torrents`
	_, err := r.db.Exec(query)
	return err
}

// List returns all active torrents
func (r *TorrentRepository) List() ([]ActiveTorrent, error) {
	rows, err := r.db.Query(`SELECT info_hash, magnet_uri, added_at FROM active_torrents ORDER BY added_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var torrents []ActiveTorrent
	for rows.Next() {
		var t ActiveTorrent
		if err := rows.Scan(&t.InfoHash, &t.MagnetURI, &t.AddedAt); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}
		torrents = append(torrents, t)
	}
	return torrents, nil
}

// Close closes the database connection
func (r *TorrentRepository) Close() error {
	return r.db.Close()
}
