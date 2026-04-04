package repository

import "context"

type GDriveClient interface {
	Upload(ctx context.Context, filePath string, filename string, onProgress func(float64)) (string, string, error)
}
