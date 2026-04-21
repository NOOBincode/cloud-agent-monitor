package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ServiceLabel struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ServiceID uuid.UUID `gorm:"type:uuid;not null;index:idx_service_label" json:"service_id"`
	Key       string    `gorm:"type:varchar(255);not null;index:idx_service_label" json:"key"`
	Value     string    `gorm:"type:varchar(255);not null;index:idx_service_label" json:"value"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (ServiceLabel) TableName() string {
	return "service_labels"
}

func (sl *ServiceLabel) BeforeCreate(tx *gorm.DB) error {
	if sl.ID == uuid.Nil {
		sl.ID = uuid.New()
	}
	return nil
}
