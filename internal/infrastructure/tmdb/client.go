package tmdb

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

const (
	baseURL           = "https://api.themoviedb.org/3"
	imageBaseW500     = "https://image.tmdb.org/t/p/w500"
	imageBaseOriginal = "https://image.tmdb.org/t/p/original"
)

// Client represents TMDB API client
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// NewClient creates a new TMDB client
func NewClient() *Client {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" || apiKey == "your_api_key_here" {
		log.Println("WARNING: TMDB_API_KEY not set!")
		log.Println("Get your free API key from: https://www.themoviedb.org/settings/api")
		log.Println("Then create a .env file with: TMDB_API_KEY=your_key")
	}
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Movie represents a movie
type Movie struct {
	ID               int     `json:"id"`
	Title            string  `json:"title"`
	OriginalTitle    string  `json:"original_title"`
	Overview         string  `json:"overview"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	ReleaseDate      string  `json:"release_date"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	Popularity       float64 `json:"popularity"`
	GenreIDs         []int   `json:"genre_ids"`
	Adult            bool    `json:"adult"`
	OriginalLanguage string  `json:"original_language"`
	MediaType        string  `json:"media_type"`
}

// TVShow represents a TV series
type TVShow struct {
	ID               int     `json:"id"`
	Name             string  `json:"name"`
	OriginalName     string  `json:"original_name"`
	Overview         string  `json:"overview"`
	PosterPath       string  `json:"poster_path"`
	BackdropPath     string  `json:"backdrop_path"`
	FirstAirDate     string  `json:"first_air_date"`
	VoteAverage      float64 `json:"vote_average"`
	VoteCount        int     `json:"vote_count"`
	Popularity       float64 `json:"popularity"`
	GenreIDs         []int   `json:"genre_ids"`
	OriginalLanguage string  `json:"original_language"`
	MediaType        string  `json:"media_type"`
}

// Genre represents a genre
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// CastMember represents a cast member
type CastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
	Order       int    `json:"order"`
}

// CrewMember represents a crew member
type CrewMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}

// Credits represents movie/tv credits
type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

// Season represents a TV season
type Season struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	SeasonNumber int    `json:"season_number"`
	EpisodeCount int    `json:"episode_count"`
	AirDate      string `json:"air_date"`
}

// Episode represents a TV episode
type Episode struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Overview      string  `json:"overview"`
	StillPath     string  `json:"still_path"`
	AirDate       string  `json:"air_date"`
	EpisodeNumber int     `json:"episode_number"`
	SeasonNumber  int     `json:"season_number"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Runtime       int     `json:"runtime"`
}

// MovieDetail represents detailed movie info
type MovieDetail struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Tagline       string  `json:"tagline"`
	Overview      string  `json:"overview"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	ReleaseDate   string  `json:"release_date"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Runtime       int     `json:"runtime"`
	Status        string  `json:"status"`
	Genres        []Genre `json:"genres"`
	Budget        int64   `json:"budget"`
	Revenue       int64   `json:"revenue"`
	ImdbID        string  `json:"imdb_id"`
	Credits       Credits `json:"credits"`
}

// TVDetail represents detailed TV show info
type TVDetail struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	OriginalName     string   `json:"original_name"`
	Tagline          string   `json:"tagline"`
	Overview         string   `json:"overview"`
	PosterPath       string   `json:"poster_path"`
	BackdropPath     string   `json:"backdrop_path"`
	FirstAirDate     string   `json:"first_air_date"`
	LastAirDate      string   `json:"last_air_date"`
	VoteAverage      float64  `json:"vote_average"`
	VoteCount        int      `json:"vote_count"`
	EpisodeRunTime   []int    `json:"episode_run_time"`
	Status           string   `json:"status"`
	Type             string   `json:"type"`
	Genres           []Genre  `json:"genres"`
	NumberOfEpisodes int      `json:"number_of_episodes"`
	NumberOfSeasons  int      `json:"number_of_seasons"`
	Seasons          []Season `json:"seasons"`
	Credits          Credits  `json:"credits"`
}

// SeasonDetail represents detailed season info with episodes
type SeasonDetail struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Overview     string    `json:"overview"`
	PosterPath   string    `json:"poster_path"`
	SeasonNumber int       `json:"season_number"`
	AirDate      string    `json:"air_date"`
	Episodes     []Episode `json:"episodes"`
}

// MovieResults represents paginated movie results
type MovieResults struct {
	Page         int     `json:"page"`
	TotalPages   int     `json:"total_pages"`
	TotalResults int     `json:"total_results"`
	Results      []Movie `json:"results"`
}

// TVResults represents paginated TV results
type TVResults struct {
	Page         int      `json:"page"`
	TotalPages   int      `json:"total_pages"`
	TotalResults int      `json:"total_results"`
	Results      []TVShow `json:"results"`
}

// MultiResults represents multi-search results
type MultiResults struct {
	Page         int           `json:"page"`
	TotalPages   int           `json:"total_pages"`
	TotalResults int           `json:"total_results"`
	Results      []interface{} `json:"results"`
}

// Helper to build full image URLs
func (c *Client) GetPosterURL(path string) string {
	if path == "" {
		return ""
	}
	return imageBaseW500 + path
}

func (c *Client) GetBackdropURL(path string) string {
	if path == "" {
		return ""
	}
	return imageBaseOriginal + path
}

// doRequest makes an HTTP request to TMDB API
func (c *Client) doRequest(endpoint string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(baseURL + endpoint)
	if err != nil {
		return nil, err
	}

	q := u.Query()
	q.Set("api_key", c.apiKey)
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	resp, err := c.httpClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("TMDB API error: %s", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// GetTrendingMovies fetches trending movies
func (c *Client) GetTrendingMovies(timeWindow string, page int) (*MovieResults, error) {
	if timeWindow == "" {
		timeWindow = "week"
	}
	data, err := c.doRequest(fmt.Sprintf("/trending/movie/%s", timeWindow), map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results MovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// GetTrendingTV fetches trending TV shows
func (c *Client) GetTrendingTV(timeWindow string, page int) (*TVResults, error) {
	if timeWindow == "" {
		timeWindow = "week"
	}
	data, err := c.doRequest(fmt.Sprintf("/trending/tv/%s", timeWindow), map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// GetPopularMovies fetches popular movies
func (c *Client) GetPopularMovies(page int) (*MovieResults, error) {
	data, err := c.doRequest("/movie/popular", map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results MovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// GetPopularTV fetches popular TV shows
func (c *Client) GetPopularTV(page int) (*TVResults, error) {
	data, err := c.doRequest("/tv/popular", map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// GetTopRatedMovies fetches top rated movies
func (c *Client) GetTopRatedMovies(page int) (*MovieResults, error) {
	data, err := c.doRequest("/movie/top_rated", map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results MovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// GetTopRatedTV fetches top rated TV shows
func (c *Client) GetTopRatedTV(page int) (*TVResults, error) {
	data, err := c.doRequest("/tv/top_rated", map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// SearchMovies searches for movies
func (c *Client) SearchMovies(query string, page int) (*MovieResults, error) {
	data, err := c.doRequest("/search/movie", map[string]string{
		"query": query,
		"page":  fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results MovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// SearchTV searches for TV shows
func (c *Client) SearchTV(query string, page int) (*TVResults, error) {
	data, err := c.doRequest("/search/tv", map[string]string{
		"query": query,
		"page":  fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// GetMovieDetail fetches movie details with credits
func (c *Client) GetMovieDetail(id int) (*MovieDetail, error) {
	data, err := c.doRequest(fmt.Sprintf("/movie/%d", id), map[string]string{
		"append_to_response": "credits",
	})
	if err != nil {
		return nil, err
	}

	var detail MovieDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// GetTVDetail fetches TV show details with credits
func (c *Client) GetTVDetail(id int) (*TVDetail, error) {
	data, err := c.doRequest(fmt.Sprintf("/tv/%d", id), map[string]string{
		"append_to_response": "credits",
	})
	if err != nil {
		return nil, err
	}

	var detail TVDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// GetTVSeason fetches TV season details with episodes
func (c *Client) GetTVSeason(tvID int, seasonNumber int) (*SeasonDetail, error) {
	data, err := c.doRequest(fmt.Sprintf("/tv/%d/season/%d", tvID, seasonNumber), nil)
	if err != nil {
		return nil, err
	}

	var detail SeasonDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

// DiscoverMovies discovers movies with filters
func (c *Client) DiscoverMovies(params map[string]string) (*MovieResults, error) {
	data, err := c.doRequest("/discover/movie", params)
	if err != nil {
		return nil, err
	}

	var results MovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// DiscoverTV discovers TV shows with filters
func (c *Client) DiscoverTV(params map[string]string) (*TVResults, error) {
	data, err := c.doRequest("/discover/tv", params)
	if err != nil {
		return nil, err
	}

	var results TVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

// GetMovieGenres fetches movie genres
func (c *Client) GetMovieGenres() ([]Genre, error) {
	data, err := c.doRequest("/genre/movie/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Genres []Genre `json:"genres"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Genres, nil
}

// GetTVGenres fetches TV genres
func (c *Client) GetTVGenres() ([]Genre, error) {
	data, err := c.doRequest("/genre/tv/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Genres []Genre `json:"genres"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Genres, nil
}
