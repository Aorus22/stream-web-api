package custom_provider

import (
	"errors"

	"github.com/google/uuid"
	cpmodel "torrent-stream/internal/model/custom_provider"
	cprepo "torrent-stream/internal/repository/custom_provider"
)

// CustomProviderUsecase handles business logic for custom providers
type CustomProviderUsecase struct {
	repo *cprepo.CustomProviderRepository
}

// NewCustomProviderUsecase creates a new custom provider usecase
func NewCustomProviderUsecase(repo *cprepo.CustomProviderRepository) *CustomProviderUsecase {
	return &CustomProviderUsecase{repo: repo}
}

// Create creates a new custom provider
func (uc *CustomProviderUsecase) Create(name, baseURL, pageType, code string) (*cpmodel.CustomProvider, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	if baseURL == "" {
		return nil, errors.New("base URL is required")
	}
	if code == "" {
		return nil, errors.New("code is required")
	}

	provider := &cpmodel.CustomProvider{
		ID:              uuid.New().String(),
		Name:            name,
		BaseURL:         baseURL,
		PageTypeDefault: pageType,
		Code:            code,
	}

	if err := uc.repo.Create(provider); err != nil {
		return nil, err
	}

	return provider, nil
}

// GetAll retrieves all custom providers
func (uc *CustomProviderUsecase) GetAll() ([]cpmodel.CustomProvider, error) {
	return uc.repo.GetAll()
}

// GetByID retrieves a custom provider by ID
func (uc *CustomProviderUsecase) GetByID(id string) (*cpmodel.CustomProvider, error) {
	if id == "" {
		return nil, errors.New("id is required")
	}

	provider, err := uc.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New("custom provider not found")
	}

	return provider, nil
}

// Update updates an existing custom provider
func (uc *CustomProviderUsecase) Update(id, name, baseURL, pageType, code string) (*cpmodel.CustomProvider, error) {
	if id == "" {
		return nil, errors.New("id is required")
	}
	if name == "" {
		return nil, errors.New("name is required")
	}
	if baseURL == "" {
		return nil, errors.New("base URL is required")
	}
	if code == "" {
		return nil, errors.New("code is required")
	}

	provider, err := uc.GetByID(id)
	if err != nil {
		return nil, err
	}

	provider.Name = name
	provider.BaseURL = baseURL
	provider.PageTypeDefault = pageType
	provider.Code = code

	if err := uc.repo.Update(provider); err != nil {
		return nil, err
	}

	return provider, nil
}

// Delete deletes a custom provider by ID
func (uc *CustomProviderUsecase) Delete(id string) error {
	if id == "" {
		return errors.New("id is required")
	}

	return uc.repo.Delete(id)
}
