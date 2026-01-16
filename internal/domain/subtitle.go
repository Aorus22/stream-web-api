package domain

// Subtitle represents a subtitle search result
type Subtitle struct {
	IDMovie         string `json:"IDMovie"`
	IDSubtitleFile  string `json:"IDSubtitleFile"`
	MovieName       string `json:"MovieName"`
	SubFileName     string `json:"SubFileName"`
	LanguageName    string `json:"LanguageName"`
	ZipDownloadLink string `json:"ZipDownloadLink"`
	SubDownloadLink string `json:"SubDownloadLink"`
}

// SubtitleCue represents a single subtitle cue with timing
type SubtitleCue struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// AutoSyncRequest represents a request for auto-sync
type AutoSyncRequest struct {
	InfoHash    string  `json:"infoHash"`
	FileIndex   int     `json:"fileIndex"`
	SubLink     string  `json:"subLink"`
	CurrentTime float64 `json:"currentTime"`
}

// AutoSyncResponse represents auto-sync result
type AutoSyncResponse struct {
	Offset float64 `json:"offset"`
	Score  float64 `json:"score"`
}
