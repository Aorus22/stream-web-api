package repository

import (
	"database/sql"
	"log"
	"time"

	domainrepo "stream-web-api/internal/domain/repository"
)

type TorrentRepository struct {
	db *sql.DB
}

type ActiveTorrent = domainrepo.ActiveTorrentRecord

func NewTorrentRepository(db *sql.DB) *TorrentRepository {
	return &TorrentRepository{db: db}
}

func (r *TorrentRepository) Add(infoHash, magnetURI string) error {
	_, err := r.db.Exec(
		`INSERT OR REPLACE INTO active_torrents (info_hash, magnet_uri, added_at) VALUES (?, ?, ?)`,
		infoHash, magnetURI, time.Now(),
	)
	return err
}

func (r *TorrentRepository) Remove(infoHash string) error {
	_, err := r.db.Exec(`DELETE FROM active_torrents WHERE info_hash = ?`, infoHash)
	return err
}

func (r *TorrentRepository) RemoveAll() error {
	_, err := r.db.Exec(`DELETE FROM active_torrents`)
	return err
}

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

func (r *TorrentRepository) SaveMetadata(infoHash, metadataJSON string) error {
	_, err := r.db.Exec(
		`INSERT OR REPLACE INTO torrent_metadata (info_hash, metadata_json, created_at) VALUES (?, ?, ?)`,
		infoHash, metadataJSON, time.Now(),
	)
	return err
}

func (r *TorrentRepository) GetMetadata(infoHash string) (string, error) {
	var jsonStr string
	err := r.db.QueryRow(`SELECT metadata_json FROM torrent_metadata WHERE info_hash = ?`, infoHash).Scan(&jsonStr)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return jsonStr, err
}
