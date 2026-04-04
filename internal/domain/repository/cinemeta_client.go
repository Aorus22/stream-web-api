package repository

import "stream-web-api/internal/domain/model"

type CinemetaClient interface {
	GetTopMovies(skip int) ([]model.Meta, error)
	GetTopSeries(skip int) ([]model.Meta, error)
	GetImdbRatingMovies(skip int) ([]model.Meta, error)
	GetImdbRatingSeries(skip int) ([]model.Meta, error)
	GetGenreMovies(genre string, skip int) ([]model.Meta, error)
	GetGenreSeries(genre string, skip int) ([]model.Meta, error)
	SearchMovies(query string) ([]model.Meta, error)
	SearchSeries(query string) ([]model.Meta, error)
	GetMovieDetail(imdbID string) (*model.Meta, error)
	GetSeriesDetail(imdbID string) (*model.Meta, error)
}
