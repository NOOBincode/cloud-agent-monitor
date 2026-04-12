package models

import (
	"database/sql"
	"time"
)

type AIModel struct {
	ID                  string    `json:"id" gorm:"primaryKey;type:char(36)"`
	Name                string    `json:"name" gorm:"type:varchar(255);not null"`
	Provider            string    `json:"provider" gorm:"type:varchar(50);not null;comment:'OTel: gen_ai.system'"`
	ModelID             string    `json:"model_id" gorm:"type:varchar(255);not null;comment:'OTel: gen_ai.request.model'"`
	Config              []byte    `json:"config" gorm:"type:json"`
	CostPerInputToken   sql.NullFloat64 `json:"cost_per_input_token" gorm:"type:decimal(20,10)"`
	CostPerOutputToken  sql.NullFloat64 `json:"cost_per_output_token" gorm:"type:decimal(20,10)"`
	Enabled             bool      `json:"enabled" gorm:"default:true"`
	CreatedAt           time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt           time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

func (AIModel) TableName() string {
	return "ai_models"
}
