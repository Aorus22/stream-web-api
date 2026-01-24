package domain

// Torrent represents a torrent with its files and stats
type Torrent struct {
	InfoHash      string  `json:"infoHash"`
	Name          string  `json:"name"`
	MagnetURI     string  `json:"magnetUri"`
	TotalLength   int64   `json:"totalLength"`
	Downloaded    int64   `json:"downloaded"`
	Progress      float64 `json:"progress"`
	Peers         int     `json:"peers"`
	DownloadSpeed int64   `json:"downloadSpeed"`
	UploadSpeed   int64   `json:"uploadSpeed"`
	AddedAt       string  `json:"addedAt"`
	Status        string  `json:"status"`
	Files         []File  `json:"files"`
}

// File represents a single file in a torrent
type File struct {
	Index          int     `json:"index"`
	Name           string  `json:"name"`
	Length         int64   `json:"length"`
	Progress       float64 `json:"progress"`
	Downloaded     int64   `json:"downloaded"`
	StreamURL      string  `json:"streamUrl"`
	PieceStart     int     `json:"pieceStart"`
	PieceEnd       int     `json:"pieceEnd"`
	PiecesReady    int     `json:"piecesReady"`
	PiecesTotal    int     `json:"piecesTotal"`
	BufferedRanges []Range `json:"bufferedRanges"`
}

// Range represents a byte range
type Range struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}
