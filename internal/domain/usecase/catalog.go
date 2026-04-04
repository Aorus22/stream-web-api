package usecase

import (
	"fmt"
	"log"
	"net/url"
	"strings"
	"sync"
	"time"

	"stream-web-api/internal/domain/model"
	domainrepo "stream-web-api/internal/domain/repository"
)

const (
	catalogMaxSkip         = 500
	catalogCacheTTLList    = 5 * time.Minute
	catalogCacheTTLDetail  = 30 * time.Minute
	catalogMaxCacheEntries = 200
)

type catalogCacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

type CatalogUsecase struct {
	client domainrepo.CinemetaClient

	mu    sync.RWMutex
	cache map[string]catalogCacheEntry
}

func NewCatalogUsecase(client domainrepo.CinemetaClient) *CatalogUsecase {
	return &CatalogUsecase{
		client: client,
		cache:  make(map[string]catalogCacheEntry),
	}
}

func (s *CatalogUsecase) GetTopMovies(skip int) ([]model.Meta, error) {
	skip = catalogClampSkip(skip)

	if cached := s.catalogGetListCache("top_movies", skip); cached != nil {
		return cached, nil
	}

	metas, err := s.client.GetTopMovies(skip)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top movies: %w", err)
	}

	result := catalogNormalizeList(metas)
	s.catalogSetListCache("top_movies", skip, result)
	return result, nil
}

func (s *CatalogUsecase) GetTopSeries(skip int) ([]model.Meta, error) {
	skip = catalogClampSkip(skip)

	if cached := s.catalogGetListCache("top_series", skip); cached != nil {
		return cached, nil
	}

	metas, err := s.client.GetTopSeries(skip)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top series: %w", err)
	}

	result := catalogNormalizeList(metas)
	s.catalogSetListCache("top_series", skip, result)
	return result, nil
}

func (s *CatalogUsecase) GetImdbRatingMovies(skip int) ([]model.Meta, error) {
	skip = catalogClampSkip(skip)

	if cached := s.catalogGetListCache("imdb_movies", skip); cached != nil {
		return cached, nil
	}

	metas, err := s.client.GetImdbRatingMovies(skip)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top rated movies: %w", err)
	}

	result := catalogNormalizeList(metas)
	s.catalogSetListCache("imdb_movies", skip, result)
	return result, nil
}

func (s *CatalogUsecase) GetImdbRatingSeries(skip int) ([]model.Meta, error) {
	skip = catalogClampSkip(skip)

	if cached := s.catalogGetListCache("imdb_series", skip); cached != nil {
		return cached, nil
	}

	metas, err := s.client.GetImdbRatingSeries(skip)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch top rated series: %w", err)
	}

	result := catalogNormalizeList(metas)
	s.catalogSetListCache("imdb_series", skip, result)
	return result, nil
}

func (s *CatalogUsecase) GetGenreMovies(genre string, skip int) ([]model.Meta, error) {
	genre = catalogSanitizeInput(genre)
	if genre == "" {
		return []model.Meta{}, nil
	}
	skip = catalogClampSkip(skip)

	cacheKey := fmt.Sprintf("genre_movies_%s", genre)
	if cached := s.catalogGetListCache(cacheKey, skip); cached != nil {
		return cached, nil
	}

	metas, err := s.client.GetGenreMovies(genre, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch genre movies (%s): %w", genre, err)
	}

	result := catalogNormalizeList(metas)
	s.catalogSetListCache(cacheKey, skip, result)
	return result, nil
}

func (s *CatalogUsecase) GetGenreSeries(genre string, skip int) ([]model.Meta, error) {
	genre = catalogSanitizeInput(genre)
	if genre == "" {
		return []model.Meta{}, nil
	}
	skip = catalogClampSkip(skip)

	cacheKey := fmt.Sprintf("genre_series_%s", genre)
	if cached := s.catalogGetListCache(cacheKey, skip); cached != nil {
		return cached, nil
	}

	metas, err := s.client.GetGenreSeries(genre, skip)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch genre series (%s): %w", genre, err)
	}

	result := catalogNormalizeList(metas)
	s.catalogSetListCache(cacheKey, skip, result)
	return result, nil
}

func (s *CatalogUsecase) SearchMovies(query string) ([]model.Meta, error) {
	query = catalogSanitizeInput(query)
	if query == "" {
		return []model.Meta{}, nil
	}

	cacheKey := fmt.Sprintf("search_movies_%s", query)
	if cached := s.catalogGetListCache(cacheKey, 0); cached != nil {
		return cached, nil
	}

	metas, err := s.client.SearchMovies(query)
	if err != nil {
		return nil, fmt.Errorf("failed to search movies: %w", err)
	}

	result := catalogNormalizeList(metas)
	s.catalogSetListCache(cacheKey, 0, result)
	return result, nil
}

func (s *CatalogUsecase) SearchSeries(query string) ([]model.Meta, error) {
	query = catalogSanitizeInput(query)
	if query == "" {
		return []model.Meta{}, nil
	}

	cacheKey := fmt.Sprintf("search_series_%s", query)
	if cached := s.catalogGetListCache(cacheKey, 0); cached != nil {
		return cached, nil
	}

	metas, err := s.client.SearchSeries(query)
	if err != nil {
		return nil, fmt.Errorf("failed to search series: %w", err)
	}

	result := catalogNormalizeList(metas)
	s.catalogSetListCache(cacheKey, 0, result)
	return result, nil
}

func (s *CatalogUsecase) GetMovieDetail(imdbID string) (*model.Meta, error) {
	imdbID = catalogSanitizeImdbID(imdbID)
	if imdbID == "" {
		return nil, fmt.Errorf("invalid imdb ID")
	}

	cacheKey := fmt.Sprintf("detail_movie_%s", imdbID)
	if cached := s.catalogGetDetailCache(cacheKey); cached != nil {
		return cached, nil
	}

	meta, err := s.client.GetMovieDetail(imdbID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch movie detail (%s): %w", imdbID, err)
	}

	s.catalogSetDetailCache(cacheKey, meta)
	return meta, nil
}

func (s *CatalogUsecase) GetSeriesDetail(imdbID string) (*model.Meta, error) {
	imdbID = catalogSanitizeImdbID(imdbID)
	if imdbID == "" {
		return nil, fmt.Errorf("invalid imdb ID")
	}

	cacheKey := fmt.Sprintf("detail_series_%s", imdbID)
	if cached := s.catalogGetDetailCache(cacheKey); cached != nil {
		return cached, nil
	}

	meta, err := s.client.GetSeriesDetail(imdbID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch series detail (%s): %w", imdbID, err)
	}

	s.catalogSetDetailCache(cacheKey, meta)
	return meta, nil
}

func (s *CatalogUsecase) catalogGetListCache(key string, skip int) []model.Meta {
	cacheKey := fmt.Sprintf("list:%s:%d", key, skip)
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.cache[cacheKey]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}

	if metas, ok := entry.data.([]model.Meta); ok {
		return metas
	}
	return nil
}

func (s *CatalogUsecase) catalogSetListCache(key string, skip int, data []model.Meta) {
	cacheKey := fmt.Sprintf("list:%s:%d", key, skip)
	s.catalogEvictIfNeeded()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[cacheKey] = catalogCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(catalogCacheTTLList),
	}
}

func (s *CatalogUsecase) catalogGetDetailCache(key string) *model.Meta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.cache[key]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}

	if meta, ok := entry.data.(*model.Meta); ok {
		return meta
	}
	return nil
}

func (s *CatalogUsecase) catalogSetDetailCache(key string, data *model.Meta) {
	s.catalogEvictIfNeeded()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache[key] = catalogCacheEntry{
		data:      data,
		expiresAt: time.Now().Add(catalogCacheTTLDetail),
	}
}

func (s *CatalogUsecase) catalogEvictIfNeeded() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cache) >= catalogMaxCacheEntries {
		now := time.Now()
		for k, entry := range s.cache {
			if now.After(entry.expiresAt) {
				delete(s.cache, k)
			}
		}
		if len(s.cache) >= catalogMaxCacheEntries {
			count := 0
			for k := range s.cache {
				delete(s.cache, k)
				count++
				if count >= catalogMaxCacheEntries/2 {
					break
				}
			}
			log.Printf("Catalog cache: evicted %d entries", count)
		}
	}
}

func catalogClampSkip(skip int) int {
	if skip < 0 {
		return 0
	}
	if skip > catalogMaxSkip {
		return catalogMaxSkip
	}
	return skip
}

func catalogSanitizeInput(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	return s
}

func catalogSanitizeImdbID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return ""
	}
	if !strings.HasPrefix(id, "tt") {
		id = "tt" + id
	}
	return id
}

func catalogNormalizeList(metas []model.Meta) []model.Meta {
	if metas == nil {
		return []model.Meta{}
	}
	return metas
}

type CatalogResult struct {
	Items   []model.Meta
	HasMore bool
}

func (s *CatalogUsecase) GetTopMoviesPaginated(skip int) (*CatalogResult, error) {
	metas, err := s.GetTopMovies(skip)
	if err != nil {
		return nil, err
	}
	return &CatalogResult{
		Items:   metas,
		HasMore: len(metas) >= 20,
	}, nil
}

func (s *CatalogUsecase) SearchMoviesRaw(query string) ([]model.Meta, error) {
	if query == "" {
		return []model.Meta{}, nil
	}
	encodedQuery := url.QueryEscape(strings.TrimSpace(query))
	return s.SearchMovies(encodedQuery)
}

func (s *CatalogUsecase) SearchSeriesRaw(query string) ([]model.Meta, error) {
	if query == "" {
		return []model.Meta{}, nil
	}
	encodedQuery := url.QueryEscape(strings.TrimSpace(query))
	return s.SearchSeries(encodedQuery)
}
