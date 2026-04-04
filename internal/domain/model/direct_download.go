package model

import "time"

type DirectDownload struct {
	ID              int        `json:"id"`
	URL             string     `json:"url"`
	Filename        string     `json:"filename"`
	Status          string     `json:"status"`
	Progress        float64    `json:"progress"`
	DownloadedBytes int64      `json:"downloadedBytes"`
	TotalBytes      int64      `json:"totalBytes"`
	AddedAt         time.Time  `json:"addedAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	FilePath        string     `json:"filePath"`
}

type DownloadProgress struct {
	ID              int     `json:"id"`
	Progress        float64 `json:"progress"`
	DownloadedBytes int64   `json:"downloadedBytes"`
	TotalBytes      int64   `json:"totalBytes"`
	Status          string  `json:"status"`
}
