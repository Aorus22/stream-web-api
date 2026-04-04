package usecase

import (
	"errors"

	"github.com/google/uuid"
	"stream-web-api/internal/domain/model"
	domainrepo "stream-web-api/internal/domain/repository"
)

type CustomProviderUsecase struct {
	repo domainrepo.CustomProviderRepository
}

func NewCustomProviderUsecase(repo domainrepo.CustomProviderRepository) *CustomProviderUsecase {
	return &CustomProviderUsecase{repo: repo}
}

func (u *CustomProviderUsecase) Create(name, baseURL, pageType, code, language string) (*model.CustomProvider, error) {
	if name == "" {
		return nil, errors.New("name is required")
	}
	if baseURL == "" {
		return nil, errors.New("base URL is required")
	}
	if code == "" {
		return nil, errors.New("code is required")
	}

	provider := &model.CustomProvider{
		ID:              uuid.New().String(),
		Name:            name,
		BaseURL:         baseURL,
		PageTypeDefault: pageType,
		Code:            code,
		Language:        language,
	}

	if err := u.repo.Create(provider); err != nil {
		return nil, err
	}

	return provider, nil
}

func (u *CustomProviderUsecase) GetAll() ([]model.CustomProvider, error) {
	return u.repo.GetAll()
}

func (u *CustomProviderUsecase) GetByID(id string) (*model.CustomProvider, error) {
	if id == "" {
		return nil, errors.New("id is required")
	}

	provider, err := u.repo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if provider == nil {
		return nil, errors.New("custom provider not found")
	}

	return provider, nil
}

func (u *CustomProviderUsecase) Update(id, name, baseURL, pageType, code, language string) (*model.CustomProvider, error) {
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

	provider, err := u.GetByID(id)
	if err != nil {
		return nil, err
	}

	provider.Name = name
	provider.BaseURL = baseURL
	provider.PageTypeDefault = pageType
	provider.Code = code
	provider.Language = language

	if err := u.repo.Update(provider); err != nil {
		return nil, err
	}

	return provider, nil
}

func (u *CustomProviderUsecase) Delete(id string) error {
	if id == "" {
		return errors.New("id is required")
	}

	return u.repo.Delete(id)
}
