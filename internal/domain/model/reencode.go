package model

type ReencodeProgress struct {
	Percent float64 `json:"percent"`
	Speed   string  `json:"speed"`
	Time    string  `json:"time"`
}

type ReencodeJob struct {
	ID         string           `json:"id"`
	Filename   string           `json:"filename"`
	Resolution string           `json:"resolution"`
	Bitrate    string           `json:"bitrate"`
	Progress   ReencodeProgress `json:"progress"`
	Status     string           `json:"status"`
}

type ReencodeJobResult struct {
	Message    string `json:"message"`
	OutputPath string `json:"outputPath"`
}

type ReencodeJobStatus struct {
	ID         string           `json:"id"`
	Filename   string           `json:"filename"`
	Resolution string           `json:"resolution"`
	Bitrate    string           `json:"bitrate"`
	Progress   ReencodeProgress `json:"progress"`
	Status     string           `json:"status"`
}

type GDriveJobStatus struct {
	ID       string  `json:"id"`
	Filename string  `json:"filename"`
	Status   string  `json:"status"`
	Progress float64 `json:"progress"`
	Link     string  `json:"link,omitempty"`
	Error    string  `json:"error,omitempty"`
}
