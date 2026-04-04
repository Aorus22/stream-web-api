package handler

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"

	uc "stream-web-api/internal/domain/usecase"

	"stream-web-api/internal/domain/model"
)

type CatalogHandler struct {
	service *uc.CatalogUsecase
}

func NewCatalogHandler(service *uc.CatalogUsecase) *CatalogHandler {
	return &CatalogHandler{service: service}
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

func (h *CatalogHandler) HandleTopMovies(c *gin.Context) {
	skip := getSkipParam(c)

	metas, err := h.service.GetTopMovies(skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metasToCatalog(metas, "movie"))
}

func (h *CatalogHandler) HandleTopSeries(c *gin.Context) {
	skip := getSkipParam(c)

	metas, err := h.service.GetTopSeries(skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metasToCatalog(metas, "series"))
}

func (h *CatalogHandler) HandleTopRatedMovies(c *gin.Context) {
	skip := getSkipParam(c)

	metas, err := h.service.GetImdbRatingMovies(skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metasToCatalog(metas, "movie"))
}

func (h *CatalogHandler) HandleTopRatedSeries(c *gin.Context) {
	skip := getSkipParam(c)

	metas, err := h.service.GetImdbRatingSeries(skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metasToCatalog(metas, "series"))
}

func (h *CatalogHandler) HandleGenreMovies(c *gin.Context) {
	genre := c.Param("genre")
	skip := getSkipParam(c)

	metas, err := h.service.GetGenreMovies(genre, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metasToCatalog(metas, "movie"))
}

func (h *CatalogHandler) HandleGenreSeries(c *gin.Context) {
	genre := c.Param("genre")
	skip := getSkipParam(c)

	metas, err := h.service.GetGenreSeries(genre, skip)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metasToCatalog(metas, "series"))
}

func (h *CatalogHandler) HandleSearchMovies(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query required"})
		return
	}

	encodedQuery := url.QueryEscape(query)

	metas, err := h.service.SearchMovies(encodedQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metasToCatalog(metas, "movie"))
}

func (h *CatalogHandler) HandleSearchSeries(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query required"})
		return
	}

	encodedQuery := url.QueryEscape(query)

	metas, err := h.service.SearchSeries(encodedQuery)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metasToCatalog(metas, "series"))
}

func (h *CatalogHandler) HandleMovieDetail(c *gin.Context) {
	imdbID := c.Param("id")
	if imdbID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IMDb ID required"})
		return
	}

	meta, err := h.service.GetMovieDetail(imdbID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metaToDetail(meta, "movie"))
}

func (h *CatalogHandler) HandleSeriesDetail(c *gin.Context) {
	imdbID := c.Param("id")
	if imdbID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "IMDb ID required"})
		return
	}

	meta, err := h.service.GetSeriesDetail(imdbID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, h.metaToDetail(meta, "series"))
}

func getSkipParam(c *gin.Context) int {
	skipStr := c.DefaultQuery("skip", "0")
	skip, err := strconv.Atoi(skipStr)
	if err != nil || skip < 0 {
		return 0
	}
	return skip
}

func (h *CatalogHandler) metasToCatalog(metas []model.Meta, mediaType string) CatalogResponse {
	items := make([]MediaItem, 0, len(metas))
	for _, m := range metas {
		items = append(items, MediaItem{
			ID:          m.ID,
			Title:       m.Name,
			Overview:    m.Description,
			Poster:      m.Poster,
			Backdrop:    m.Background,
			ReleaseInfo: m.ReleaseInfo,
			Year:        m.Year,
			Rating:      m.IMDbRating,
			Runtime:     m.Runtime,
			MediaType:   mediaType,
			Genres:      m.Genres,
		})
	}
	return CatalogResponse{
		Results: items,
		HasMore: len(items) >= 20,
	}
}

func (h *CatalogHandler) metaToDetail(meta *model.Meta, mediaType string) MediaDetail {
	trailers := make([]TrailerInfo, 0, len(meta.Trailers))
	for _, t := range meta.Trailers {
		trailers = append(trailers, TrailerInfo{
			Source: t.Source,
			Type:   t.Type,
		})
	}

	episodes := make([]EpisodeInfo, 0)
	if mediaType == "series" {
		for _, v := range meta.Videos {
			episodes = append(episodes, EpisodeInfo{
				ID:        v.ID,
				Title:     v.Title,
				Season:    v.Season,
				Episode:   v.Episode,
				Released:  v.Released,
				Thumbnail: v.Thumbnail,
				Overview:  v.Overview,
			})
		}
	}

	return MediaDetail{
		ID:          meta.ID,
		Title:       meta.Name,
		Overview:    meta.Description,
		Poster:      meta.Poster,
		Backdrop:    meta.Background,
		Logo:        meta.Logo,
		ReleaseInfo: meta.ReleaseInfo,
		Year:        meta.Year,
		Rating:      meta.IMDbRating,
		Runtime:     meta.Runtime,
		Genres:      meta.Genres,
		Cast:        meta.Cast,
		Director:    meta.Director,
		Writer:      meta.Writer,
		MediaType:   mediaType,
		Trailers:    trailers,
		Episodes:    episodes,
	}
}
