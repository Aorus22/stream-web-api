package repository

import "stream-web-api/internal/domain/model"

type CustomProviderRepository interface {
	Create(provider *model.CustomProvider) error
	GetAll() ([]model.CustomProvider, error)
	GetByID(id string) (*model.CustomProvider, error)
	Update(provider *model.CustomProvider) error
	Delete(id string) error
}
