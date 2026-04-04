package repository

import (
	"stream-web-api/internal/domain/model"
)

type DirectDownloadRepository interface {
	Create(url string, filename string, status string, filePath string) (int, error)
	Get(id int) (*model.DirectDownload, error)
	List() ([]model.DirectDownload, error)
	UpdateProgress(id int, progress float64, downloadedBytes int64, totalBytes int64) error
	MarkCompleted(id int, filePath string, totalBytes int64) error
	MarkFailed(id int) error
	Delete(id int) error
	DeleteAll() error
}
