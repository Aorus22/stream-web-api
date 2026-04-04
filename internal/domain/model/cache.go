package model

type CachedFileWithType struct {
	Name       string  `json:"name"`
	Path       string  `json:"path"`
	Size       int64   `json:"size"`
	Type       string  `json:"type"`
	InfoHash   string  `json:"infoHash,omitempty"`
	FileIndex  int     `json:"fileIndex,omitempty"`
	DownloadID int     `json:"downloadId,omitempty"`
	Progress   float64 `json:"progress,omitempty"`
	Status     string  `json:"status,omitempty"`
	StreamURL  string  `json:"streamUrl"`
	CanPlay    bool    `json:"canPlay"`
}

type CacheStats struct {
	TotalSize int64 `json:"totalSize"`
	FileCount int   `json:"fileCount"`
	CacheDir  string `json:"cacheDir"`
}

type GDriveUploadParams struct {
	InfoHash   string `json:"infoHash"`
	FileIndex  int    `json:"fileIndex"`
	DownloadID int    `json:"downloadId"`
	ExportPath string `json:"exportPath"`
}

type GDriveJob struct {
	ID       string  `json:"id"`
	Filename string  `json:"filename"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Link     string  `json:"link,omitempty"`
	Error    string  `json:"error,omitempty"`
}

type TasksSSEEvent struct {
	GDrive   []GDriveJob   `json:"gdrive"`
	Reencode []interface{} `json:"reencode"`
}
