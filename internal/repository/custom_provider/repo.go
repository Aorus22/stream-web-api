package custom_provider

import (
	"errors"

	"gorm.io/gorm"
	"torrent-stream/internal/model/custom_provider"
)

// CustomProviderRepository handles CRUD operations for custom providers
type CustomProviderRepository struct {
	db *gorm.DB
}

// NewCustomProviderRepository creates a new custom provider repository
func NewCustomProviderRepository(db *gorm.DB) *CustomProviderRepository {
	return &CustomProviderRepository{db: db}
}

// Create creates a new custom provider
func (r *CustomProviderRepository) Create(provider *custom_provider.CustomProvider) error {
	return r.db.Create(provider).Error
}

// GetAll retrieves all custom providers
func (r *CustomProviderRepository) GetAll() ([]custom_provider.CustomProvider, error) {
	var providers []custom_provider.CustomProvider
	err := r.db.Order("created_at DESC").Find(&providers).Error
	return providers, err
}

// GetByID retrieves a custom provider by ID
func (r *CustomProviderRepository) GetByID(id string) (*custom_provider.CustomProvider, error) {
	var provider custom_provider.CustomProvider
	err := r.db.Where("id = ?", id).First(&provider).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &provider, nil
}

// Update updates an existing custom provider
func (r *CustomProviderRepository) Update(provider *custom_provider.CustomProvider) error {
	return r.db.Save(provider).Error
}

// Delete deletes a custom provider by ID
func (r *CustomProviderRepository) Delete(id string) error {
	return r.db.Delete(&custom_provider.CustomProvider{}, "id = ?", id).Error
}
