package model

type MediaInfo struct {
	Duration  float64         `json:"duration"`
	Subtitles []SubtitleStream `json:"subtitles"`
}

type HLSPlaylistResult struct {
	Duration      float64 `json:"duration"`
	TotalSegments int     `json:"totalSegments"`
	Playlist      string  `json:"playlist"`
}

type StreamResult struct {
	ContentType    string
	StatusCode     int
	FileSize       int64
	ContentStart   int64
	ContentEnd     int64
	ContentLength  int64
	IsRangeRequest bool
	Filename       string
}
