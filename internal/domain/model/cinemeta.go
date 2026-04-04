package model

type Meta struct {
	ID            string         `json:"id"`
	Type          string         `json:"type"`
	Name          string         `json:"name"`
	Poster        string         `json:"poster"`
	Background    string         `json:"background"`
	Logo          string         `json:"logo"`
	Description   string         `json:"description"`
	ReleaseInfo   string         `json:"releaseInfo"`
	IMDbRating    string         `json:"imdbRating"`
	Runtime       string         `json:"runtime"`
	Genres        []string       `json:"genres"`
	Cast          []string       `json:"cast"`
	Director      []string       `json:"director"`
	Writer        []string       `json:"writer"`
	Year          string         `json:"year"`
	Trailers      []Trailer      `json:"trailers"`
	Links         []Link         `json:"links"`
	Videos        []Video        `json:"videos"`
	BehaviorHints *BehaviorHints `json:"behaviorHints"`
}

type Trailer struct {
	Source string `json:"source"`
	Type   string `json:"type"`
}

type Link struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	URL      string `json:"url"`
}

type Video struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Released  string `json:"released"`
	Season    int    `json:"season"`
	Episode   int    `json:"episode"`
	Thumbnail string `json:"thumbnail"`
	Overview  string `json:"overview"`
}

type BehaviorHints struct {
	DefaultVideoID string `json:"defaultVideoId"`
	HasSchedule    bool   `json:"hasScheduledVideos"`
}

type MediaItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Overview    string   `json:"overview"`
	Poster      string   `json:"poster"`
	Backdrop    string   `json:"backdrop"`
	ReleaseInfo string   `json:"releaseInfo"`
	Year        string   `json:"year"`
	Rating      string   `json:"rating"`
	Runtime     string   `json:"runtime"`
	MediaType   string   `json:"mediaType"`
	Genres      []string `json:"genres"`
}

type MediaDetail struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Overview    string        `json:"overview"`
	Poster      string        `json:"poster"`
	Backdrop    string        `json:"backdrop"`
	Logo        string        `json:"logo"`
	ReleaseInfo string        `json:"releaseInfo"`
	Year        string        `json:"year"`
	Rating      string        `json:"rating"`
	Runtime     string        `json:"runtime"`
	Genres      []string      `json:"genres"`
	Cast        []string      `json:"cast"`
	Director    []string      `json:"director"`
	Writer      []string      `json:"writer"`
	MediaType   string        `json:"mediaType"`
	Trailers    []TrailerInfo `json:"trailers"`
	Episodes    []EpisodeInfo `json:"episodes,omitempty"`
}

type TrailerInfo struct {
	Source string `json:"source"`
	Type   string `json:"type"`
}

type EpisodeInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Season    int    `json:"season"`
	Episode   int    `json:"episode"`
	Released  string `json:"released"`
	Thumbnail string `json:"thumbnail"`
	Overview  string `json:"overview"`
}

type CatalogResponse struct {
	Results []MediaItem `json:"results"`
	HasMore bool        `json:"hasMore"`
}
