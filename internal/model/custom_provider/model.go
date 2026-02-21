package custom_provider

import "time"

// CustomProvider represents a custom JavaScript scraper configuration
type CustomProvider struct {
	ID              string    `json:"id" gorm:"primaryKey"`
	Name            string    `json:"name" gorm:"not null"`
	BaseURL         string    `json:"baseUrl" gorm:"not null"`
	PageTypeDefault string    `json:"pageType" gorm:"column:page_type_default;not null;default:list"`
	Code            string    `json:"code" gorm:"not null;type:text"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
