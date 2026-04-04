package repository

import "stream-web-api/internal/domain/model"

type OpenSubtitlesClient interface {
	Search(query, lang string) ([]model.Subtitle, error)
}
