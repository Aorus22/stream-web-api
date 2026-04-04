package repository

import (
	"errors"

	"gorm.io/gorm"
	"stream-web-api/internal/domain/model"
)

type CustomProviderRepository struct {
	db *gorm.DB
}

func NewCustomProviderRepository(db *gorm.DB) *CustomProviderRepository {
	return &CustomProviderRepository{db: db}
}

func (r *CustomProviderRepository) Create(provider *model.CustomProvider) error {
	return r.db.Create(provider).Error
}

func (r *CustomProviderRepository) GetAll() ([]model.CustomProvider, error) {
	var providers []model.CustomProvider
	err := r.db.Order("created_at DESC").Find(&providers).Error
	return providers, err
}

func (r *CustomProviderRepository) GetByID(id string) (*model.CustomProvider, error) {
	var provider model.CustomProvider
	err := r.db.Where("id = ?", id).First(&provider).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

func (r *CustomProviderRepository) Update(provider *model.CustomProvider) error {
	return r.db.Save(provider).Error
}

func (r *CustomProviderRepository) Delete(id string) error {
	return r.db.Delete(&model.CustomProvider{}, "id = ?", id).Error
}
