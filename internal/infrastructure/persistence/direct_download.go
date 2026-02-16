package persistence

import (
	"database/sql"
	"path/filepath"
	"time"

	"torrent-stream/internal/domain"

	_ "modernc.org/sqlite"
)

type DirectDownloadRepository struct {
	db *sql.DB
}

func NewDirectDownloadRepository(cacheDir string) (*DirectDownloadRepository, error) {
	dbPath := filepath.Join(cacheDir, "torrents.db")

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	query := `
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
	`
	if _, err := db.Exec(query); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &DirectDownloadRepository{db: db}, nil
}

func (r *DirectDownloadRepository) Create(url string, filename string, status string, filePath string) (int, error) {
	query := `
	INSERT INTO direct_downloads (url, filename, status, progress, downloaded_bytes, total_bytes, added_at, completed_at, file_path)
	VALUES (?, ?, ?, 0, 0, 0, ?, NULL, ?)
	`
	res, err := r.db.Exec(query, url, filename, status, time.Now(), filePath)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (r *DirectDownloadRepository) Get(id int) (*domain.DirectDownload, error) {
	query := `
	SELECT id, url, filename, status, progress, downloaded_bytes, total_bytes, added_at, completed_at, file_path
	FROM direct_downloads
	WHERE id = ?
	`

	var (
		dl          domain.DirectDownload
		completedAt sql.NullTime
		filePath    sql.NullString
	)

	if err := r.db.QueryRow(query, id).Scan(
		&dl.ID,
		&dl.URL,
		&dl.Filename,
		&dl.Status,
		&dl.Progress,
		&dl.DownloadedBytes,
		&dl.TotalBytes,
		&dl.AddedAt,
		&completedAt,
		&filePath,
	); err != nil {
		return nil, err
	}

	if completedAt.Valid {
		t := completedAt.Time
		dl.CompletedAt = &t
	}
	if filePath.Valid {
		dl.FilePath = filePath.String
	}

	return &dl, nil
}

func (r *DirectDownloadRepository) List() ([]domain.DirectDownload, error) {
	rows, err := r.db.Query(`
		SELECT id, url, filename, status, progress, downloaded_bytes, total_bytes, added_at, completed_at, file_path
		FROM direct_downloads
		ORDER BY added_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []domain.DirectDownload
	for rows.Next() {
		var (
			dl          domain.DirectDownload
			completedAt sql.NullTime
			filePath    sql.NullString
		)
		if err := rows.Scan(
			&dl.ID,
			&dl.URL,
			&dl.Filename,
			&dl.Status,
			&dl.Progress,
			&dl.DownloadedBytes,
			&dl.TotalBytes,
			&dl.AddedAt,
			&completedAt,
			&filePath,
		); err != nil {
			continue
		}

		if completedAt.Valid {
			t := completedAt.Time
			dl.CompletedAt = &t
		}
		if filePath.Valid {
			dl.FilePath = filePath.String
		}

		out = append(out, dl)
	}
	return out, nil
}

func (r *DirectDownloadRepository) UpdateProgress(id int, progress float64, downloadedBytes int64, totalBytes int64) error {
	query := `
	UPDATE direct_downloads
	SET progress = ?, downloaded_bytes = ?, total_bytes = ?
	WHERE id = ?
	`
	_, err := r.db.Exec(query, progress, downloadedBytes, totalBytes, id)
	return err
}

func (r *DirectDownloadRepository) MarkCompleted(id int, filePath string, totalBytes int64) error {
	now := time.Now()
	query := `
	UPDATE direct_downloads
	SET status = 'completed', progress = 100, downloaded_bytes = ?, total_bytes = ?, completed_at = ?, file_path = ?
	WHERE id = ?
	`
	_, err := r.db.Exec(query, totalBytes, totalBytes, now, filePath, id)
	return err
}

func (r *DirectDownloadRepository) MarkFailed(id int) error {
	query := `
	UPDATE direct_downloads
	SET status = 'failed'
	WHERE id = ?
	`
	_, err := r.db.Exec(query, id)
	return err
}

func (r *DirectDownloadRepository) Delete(id int) error {
	_, err := r.db.Exec(`DELETE FROM direct_downloads WHERE id = ?`, id)
	return err
}

func (r *DirectDownloadRepository) DeleteAll() error {
	_, err := r.db.Exec(`DELETE FROM direct_downloads`)
	return err
}

func (r *DirectDownloadRepository) Close() error {
	return r.db.Close()
}

