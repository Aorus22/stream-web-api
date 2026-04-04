package repository

import (
	"database/sql"
	"time"

	"stream-web-api/internal/domain/model"
)

type DirectDownloadRepository struct {
	db *sql.DB
}

func NewDirectDownloadRepository(db *sql.DB) *DirectDownloadRepository {
	return &DirectDownloadRepository{db: db}
}

func (r *DirectDownloadRepository) Create(url string, filename string, status string, filePath string) (int, error) {
	res, err := r.db.Exec(
		`INSERT INTO direct_downloads (url, filename, status, progress, downloaded_bytes, total_bytes, added_at, completed_at, file_path)
		VALUES (?, ?, ?, 0, 0, 0, ?, NULL, ?)`,
		url, filename, status, time.Now(), filePath,
	)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

func (r *DirectDownloadRepository) Get(id int) (*model.DirectDownload, error) {
	var (
		dl          model.DirectDownload
		completedAt sql.NullTime
		filePath    sql.NullString
	)

	if err := r.db.QueryRow(
		`SELECT id, url, filename, status, progress, downloaded_bytes, total_bytes, added_at, completed_at, file_path
		FROM direct_downloads WHERE id = ?`, id,
	).Scan(&dl.ID, &dl.URL, &dl.Filename, &dl.Status, &dl.Progress, &dl.DownloadedBytes, &dl.TotalBytes, &dl.AddedAt, &completedAt, &filePath); err != nil {
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

func (r *DirectDownloadRepository) List() ([]model.DirectDownload, error) {
	rows, err := r.db.Query(
		`SELECT id, url, filename, status, progress, downloaded_bytes, total_bytes, added_at, completed_at, file_path
		FROM direct_downloads ORDER BY added_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.DirectDownload
	for rows.Next() {
		var (
			dl          model.DirectDownload
			completedAt sql.NullTime
			filePath    sql.NullString
		)
		if err := rows.Scan(&dl.ID, &dl.URL, &dl.Filename, &dl.Status, &dl.Progress, &dl.DownloadedBytes, &dl.TotalBytes, &dl.AddedAt, &completedAt, &filePath); err != nil {
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
	_, err := r.db.Exec(
		`UPDATE direct_downloads SET progress = ?, downloaded_bytes = ?, total_bytes = ? WHERE id = ?`,
		progress, downloadedBytes, totalBytes, id,
	)
	return err
}

func (r *DirectDownloadRepository) MarkCompleted(id int, filePath string, totalBytes int64) error {
	_, err := r.db.Exec(
		`UPDATE direct_downloads SET status = 'completed', progress = 100, downloaded_bytes = ?, total_bytes = ?, completed_at = ?, file_path = ? WHERE id = ?`,
		totalBytes, totalBytes, time.Now(), filePath, id,
	)
	return err
}

func (r *DirectDownloadRepository) MarkFailed(id int) error {
	_, err := r.db.Exec(`UPDATE direct_downloads SET status = 'failed' WHERE id = ?`, id)
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
