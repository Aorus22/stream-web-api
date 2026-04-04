package repository

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
	tmdbBaseURL           = "https://api.themoviedb.org/3"
	tmdbImageBaseW500     = "https://image.tmdb.org/t/p/w500"
	tmdbImageBaseOriginal = "https://image.tmdb.org/t/p/original"
)

type TMDBClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewTMDBClient() *TMDBClient {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" || apiKey == "your_api_key_here" {
		log.Println("WARNING: TMDB_API_KEY not set!")
		log.Println("Get your free API key from: https://www.themoviedb.org/settings/api")
		log.Println("Then create a .env file with: TMDB_API_KEY=your_key")
	}
	return &TMDBClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type TMDBMovie struct {
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

type TMDBTVShow struct {
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

type TMDBGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type TMDBCastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	ProfilePath string `json:"profile_path"`
	Order       int    `json:"order"`
}

type TMDBCrewMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
}

type TMDBCredits struct {
	Cast []TMDBCastMember `json:"cast"`
	Crew []TMDBCrewMember `json:"crew"`
}

type TMDBSeason struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	SeasonNumber int    `json:"season_number"`
	EpisodeCount int    `json:"episode_count"`
	AirDate      string `json:"air_date"`
}

type TMDBEpisode struct {
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

type TMDBMovieDetail struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	OriginalTitle string    `json:"original_title"`
	Tagline       string    `json:"tagline"`
	Overview      string    `json:"overview"`
	PosterPath    string    `json:"poster_path"`
	BackdropPath  string    `json:"backdrop_path"`
	ReleaseDate   string    `json:"release_date"`
	VoteAverage   float64   `json:"vote_average"`
	VoteCount     int       `json:"vote_count"`
	Runtime       int       `json:"runtime"`
	Status        string    `json:"status"`
	Genres        []TMDBGenre `json:"genres"`
	Budget        int64     `json:"budget"`
	Revenue       int64     `json:"revenue"`
	ImdbID        string    `json:"imdb_id"`
	Credits       TMDBCredits `json:"credits"`
}

type TMDBTVDetail struct {
	ID               int         `json:"id"`
	Name             string      `json:"name"`
	OriginalName     string      `json:"original_name"`
	Tagline          string      `json:"tagline"`
	Overview         string      `json:"overview"`
	PosterPath       string      `json:"poster_path"`
	BackdropPath     string      `json:"backdrop_path"`
	FirstAirDate     string      `json:"first_air_date"`
	LastAirDate      string      `json:"last_air_date"`
	VoteAverage      float64     `json:"vote_average"`
	VoteCount        int         `json:"vote_count"`
	EpisodeRunTime   []int       `json:"episode_run_time"`
	Status           string      `json:"status"`
	Type             string      `json:"type"`
	Genres           []TMDBGenre `json:"genres"`
	NumberOfEpisodes int         `json:"number_of_episodes"`
	NumberOfSeasons  int         `json:"number_of_seasons"`
	Seasons          []TMDBSeason `json:"seasons"`
	Credits          TMDBCredits `json:"credits"`
}

type TMDBSeasonDetail struct {
	ID           int          `json:"id"`
	Name         string       `json:"name"`
	Overview     string       `json:"overview"`
	PosterPath   string       `json:"poster_path"`
	SeasonNumber int          `json:"season_number"`
	AirDate      string       `json:"air_date"`
	Episodes     []TMDBEpisode `json:"episodes"`
}

type TMDBMovieResults struct {
	Page         int          `json:"page"`
	TotalPages   int          `json:"total_pages"`
	TotalResults int          `json:"total_results"`
	Results      []TMDBMovie  `json:"results"`
}

type TMDBTVResults struct {
	Page         int          `json:"page"`
	TotalPages   int          `json:"total_pages"`
	TotalResults int          `json:"total_results"`
	Results      []TMDBTVShow `json:"results"`
}

type TMDBMultiResults struct {
	Page         int           `json:"page"`
	TotalPages   int           `json:"total_pages"`
	TotalResults int           `json:"total_results"`
	Results      []interface{} `json:"results"`
}

func (c *TMDBClient) GetPosterURL(path string) string {
	if path == "" {
		return ""
	}
	return tmdbImageBaseW500 + path
}

func (c *TMDBClient) GetBackdropURL(path string) string {
	if path == "" {
		return ""
	}
	return tmdbImageBaseOriginal + path
}

func (c *TMDBClient) doRequest(endpoint string, params map[string]string) ([]byte, error) {
	u, err := url.Parse(tmdbBaseURL + endpoint)
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

func (c *TMDBClient) GetTrendingMovies(timeWindow string, page int) (*TMDBMovieResults, error) {
	if timeWindow == "" {
		timeWindow = "week"
	}
	data, err := c.doRequest(fmt.Sprintf("/trending/movie/%s", timeWindow), map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TMDBMovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) GetTrendingTV(timeWindow string, page int) (*TMDBTVResults, error) {
	if timeWindow == "" {
		timeWindow = "week"
	}
	data, err := c.doRequest(fmt.Sprintf("/trending/tv/%s", timeWindow), map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TMDBTVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) GetPopularMovies(page int) (*TMDBMovieResults, error) {
	data, err := c.doRequest("/movie/popular", map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TMDBMovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) GetPopularTV(page int) (*TMDBTVResults, error) {
	data, err := c.doRequest("/tv/popular", map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TMDBTVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) GetTopRatedMovies(page int) (*TMDBMovieResults, error) {
	data, err := c.doRequest("/movie/top_rated", map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TMDBMovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) GetTopRatedTV(page int) (*TMDBTVResults, error) {
	data, err := c.doRequest("/tv/top_rated", map[string]string{
		"page": fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TMDBTVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) SearchMovies(query string, page int) (*TMDBMovieResults, error) {
	data, err := c.doRequest("/search/movie", map[string]string{
		"query": query,
		"page":  fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TMDBMovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) SearchTV(query string, page int) (*TMDBTVResults, error) {
	data, err := c.doRequest("/search/tv", map[string]string{
		"query": query,
		"page":  fmt.Sprintf("%d", page),
	})
	if err != nil {
		return nil, err
	}

	var results TMDBTVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) GetMovieDetail(id int) (*TMDBMovieDetail, error) {
	data, err := c.doRequest(fmt.Sprintf("/movie/%d", id), map[string]string{
		"append_to_response": "credits",
	})
	if err != nil {
		return nil, err
	}

	var detail TMDBMovieDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (c *TMDBClient) GetTVDetail(id int) (*TMDBTVDetail, error) {
	data, err := c.doRequest(fmt.Sprintf("/tv/%d", id), map[string]string{
		"append_to_response": "credits",
	})
	if err != nil {
		return nil, err
	}

	var detail TMDBTVDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (c *TMDBClient) GetTVSeason(tvID int, seasonNumber int) (*TMDBSeasonDetail, error) {
	data, err := c.doRequest(fmt.Sprintf("/tv/%d/season/%d", tvID, seasonNumber), nil)
	if err != nil {
		return nil, err
	}

	var detail TMDBSeasonDetail
	if err := json.Unmarshal(data, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (c *TMDBClient) DiscoverMovies(params map[string]string) (*TMDBMovieResults, error) {
	data, err := c.doRequest("/discover/movie", params)
	if err != nil {
		return nil, err
	}

	var results TMDBMovieResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) DiscoverTV(params map[string]string) (*TMDBTVResults, error) {
	data, err := c.doRequest("/discover/tv", params)
	if err != nil {
		return nil, err
	}

	var results TMDBTVResults
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, err
	}
	return &results, nil
}

func (c *TMDBClient) GetMovieGenres() ([]TMDBGenre, error) {
	data, err := c.doRequest("/genre/movie/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Genres []TMDBGenre `json:"genres"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Genres, nil
}

func (c *TMDBClient) GetTVGenres() ([]TMDBGenre, error) {
	data, err := c.doRequest("/genre/tv/list", nil)
	if err != nil {
		return nil, err
	}

	var result struct {
		Genres []TMDBGenre `json:"genres"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result.Genres, nil
}
