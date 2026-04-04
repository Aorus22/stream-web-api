package model

import "time"

type CustomProvider struct {
	ID              string    `json:"id" gorm:"primaryKey"`
	Name            string    `json:"name" gorm:"not null"`
	BaseURL         string    `json:"baseUrl" gorm:"not null"`
	PageTypeDefault string    `json:"pageType" gorm:"column:page_type_default;not null;default:list"`
	Language        string    `json:"language" gorm:"not null;default:lua"`
	Code            string    `json:"code" gorm:"not null;type:text"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}
