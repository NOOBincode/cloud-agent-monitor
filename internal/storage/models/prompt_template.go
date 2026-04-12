package models

import (
	"database/sql"
	"time"
)

type PromptTemplate struct {
	ID          string    `json:"id" gorm:"primaryKey;type:char(36)"`
	Name        string    `json:"name" gorm:"type:varchar(255);unique;not null"`
	Description sql.NullString `json:"description" gorm:"type:text"`
	Template    string    `json:"template" gorm:"type:text;not null"`
	Variables   []byte    `json:"variables" gorm:"type:json"`
	Version     int       `json:"version" gorm:"default:1"`
	Labels      []byte    `json:"labels" gorm:"type:json"`
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (PromptTemplate) TableName() string {
	return "prompt_templates"
}
