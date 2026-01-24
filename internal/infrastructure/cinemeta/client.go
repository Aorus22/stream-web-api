package cinemeta

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL   = "https://v3-cinemeta.strem.io"
	imageBase = "https://images.metahub.space"
)

// Client represents Cinemeta API client (Stremio's public metadata API)
type Client struct {
	httpClient *http.Client
}

// NewClient creates a new Cinemeta client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Meta represents media metadata
type Meta struct {
	ID            string         `json:"id"`   // IMDb ID
	Type          string         `json:"type"` // "movie" or "series"
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

// CatalogResponse represents catalog API response
type CatalogResponse struct {
	Metas []Meta `json:"metas"`
}

// MetaResponse represents meta detail API response
type MetaResponse struct {
	Meta Meta `json:"meta"`
}

// doRequest makes HTTP request to Cinemeta API
func (c *Client) doRequest(endpoint string) ([]byte, error) {
	url := baseURL + endpoint

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Cinemeta API error: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// GetTopMovies fetches top/popular movies
func (c *Client) GetTopMovies(skip int) ([]Meta, error) {
	endpoint := "/catalog/movie/top.json"
	if skip > 0 {
		endpoint = fmt.Sprintf("/catalog/movie/top/skip=%d.json", skip)
	}

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}

// GetTopSeries fetches top/popular TV series
func (c *Client) GetTopSeries(skip int) ([]Meta, error) {
	endpoint := "/catalog/series/top.json"
	if skip > 0 {
		endpoint = fmt.Sprintf("/catalog/series/top/skip=%d.json", skip)
	}

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}

// GetYearMovies fetches movies by year
func (c *Client) GetYearMovies(year string, skip int) ([]Meta, error) {
	endpoint := fmt.Sprintf("/catalog/movie/year/genre=%s.json", year)
	if skip > 0 {
		endpoint = fmt.Sprintf("/catalog/movie/year/genre=%s&skip=%d.json", year, skip)
	}

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}

// GetYearSeries fetches series by year
func (c *Client) GetYearSeries(year string, skip int) ([]Meta, error) {
	endpoint := fmt.Sprintf("/catalog/series/year/genre=%s.json", year)
	if skip > 0 {
		endpoint = fmt.Sprintf("/catalog/series/year/genre=%s&skip=%d.json", year, skip)
	}

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}

// GetImdbRatingMovies fetches movies sorted by IMDB rating
func (c *Client) GetImdbRatingMovies(skip int) ([]Meta, error) {
	endpoint := "/catalog/movie/imdbRating.json"
	if skip > 0 {
		endpoint = fmt.Sprintf("/catalog/movie/imdbRating/skip=%d.json", skip)
	}

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}

// GetImdbRatingSeries fetches series sorted by IMDB rating
func (c *Client) GetImdbRatingSeries(skip int) ([]Meta, error) {
	endpoint := "/catalog/series/imdbRating.json"
	if skip > 0 {
		endpoint = fmt.Sprintf("/catalog/series/imdbRating/skip=%d.json", skip)
	}

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}

// SearchMovies searches for movies
func (c *Client) SearchMovies(query string) ([]Meta, error) {
	endpoint := fmt.Sprintf("/catalog/movie/top/search=%s.json", query)

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}

// SearchSeries searches for TV series
func (c *Client) SearchSeries(query string) ([]Meta, error) {
	endpoint := fmt.Sprintf("/catalog/series/top/search=%s.json", query)

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}

// GetMovieDetail fetches movie details by IMDb ID
func (c *Client) GetMovieDetail(imdbID string) (*Meta, error) {
	endpoint := fmt.Sprintf("/meta/movie/%s.json", imdbID)

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response MetaResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response.Meta, nil
}

// GetSeriesDetail fetches series details by IMDb ID (includes episodes)
func (c *Client) GetSeriesDetail(imdbID string) (*Meta, error) {
	endpoint := fmt.Sprintf("/meta/series/%s.json", imdbID)

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response MetaResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response.Meta, nil
}

// GetGenreMovies fetches movies by genre
func (c *Client) GetGenreMovies(genre string, skip int) ([]Meta, error) {
	endpoint := fmt.Sprintf("/catalog/movie/top/genre=%s.json", genre)
	if skip > 0 {
		endpoint = fmt.Sprintf("/catalog/movie/top/genre=%s&skip=%d.json", genre, skip)
	}

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}

// GetGenreSeries fetches series by genre
func (c *Client) GetGenreSeries(genre string, skip int) ([]Meta, error) {
	endpoint := fmt.Sprintf("/catalog/series/top/genre=%s.json", genre)
	if skip > 0 {
		endpoint = fmt.Sprintf("/catalog/series/top/genre=%s&skip=%d.json", genre, skip)
	}

	data, err := c.doRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var response CatalogResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return response.Metas, nil
}
