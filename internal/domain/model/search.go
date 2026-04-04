package model

type SearchResult struct {
	Name         string `json:"name"`
	Magnet       string `json:"magnet"`
	Poster       string `json:"poster"`
	Category     string `json:"category"`
	Type         string `json:"type"`
	Language     string `json:"language"`
	Size         string `json:"size"`
	UploadedBy   string `json:"uploadedBy"`
	Downloads    string `json:"downloads"`
	LastChecked  string `json:"lastChecked"`
	DateUploaded string `json:"dateUploaded"`
	Seeders      string `json:"seeders"`
	Leechers     string `json:"leechers"`
	URL          string `json:"url"`
}
