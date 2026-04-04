package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"stream-web-api/internal/domain/model"
)

const (
	cinemetaBaseURL   = "https://v3-cinemeta.strem.io"
	cinemetaImageBase = "https://images.metahub.space"
)

type CinemetaClient struct {
	httpClient *http.Client
}

func NewCinemetaClient() *CinemetaClient {
	return &CinemetaClient{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

type Meta = model.Meta
type Trailer = model.Trailer
type Link = model.Link
type Video = model.Video
type BehaviorHints = model.BehaviorHints

type CatalogResponse struct {
	Metas []Meta `json:"metas"`
}

type MetaResponse struct {
	Meta Meta `json:"meta"`
}

func (c *CinemetaClient) doRequest(endpoint string) ([]byte, error) {
	url := cinemetaBaseURL + endpoint

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

func (c *CinemetaClient) GetTopMovies(skip int) ([]Meta, error) {
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

func (c *CinemetaClient) GetTopSeries(skip int) ([]Meta, error) {
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

func (c *CinemetaClient) GetYearMovies(year string, skip int) ([]Meta, error) {
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

func (c *CinemetaClient) GetYearSeries(year string, skip int) ([]Meta, error) {
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

func (c *CinemetaClient) GetImdbRatingMovies(skip int) ([]Meta, error) {
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

func (c *CinemetaClient) GetImdbRatingSeries(skip int) ([]Meta, error) {
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

func (c *CinemetaClient) SearchMovies(query string) ([]Meta, error) {
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

func (c *CinemetaClient) SearchSeries(query string) ([]Meta, error) {
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

func (c *CinemetaClient) GetMovieDetail(imdbID string) (*Meta, error) {
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

func (c *CinemetaClient) GetSeriesDetail(imdbID string) (*Meta, error) {
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

func (c *CinemetaClient) GetGenreMovies(genre string, skip int) ([]Meta, error) {
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

func (c *CinemetaClient) GetGenreSeries(genre string, skip int) ([]Meta, error) {
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
