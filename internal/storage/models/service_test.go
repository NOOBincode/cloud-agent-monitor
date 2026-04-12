package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestService_TableName(t *testing.T) {
	svc := Service{}
	assert.Equal(t, "services", svc.TableName())
}

func TestService_BeforeCreate(t *testing.T) {
	tests := []struct {
		name         string
		initialID    uuid.UUID
		expectedNew  bool
	}{
		{
			name:        "generates new uuid when nil",
			initialID:   uuid.Nil,
			expectedNew: true,
		},
		{
			name:        "keeps existing uuid",
			initialID:   uuid.New(),
			expectedNew: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{ID: tt.initialID}
			err := svc.BeforeCreate(nil)

			assert.NoError(t, err)
			assert.NotEqual(t, uuid.Nil, svc.ID)

			if !tt.expectedNew {
				assert.Equal(t, tt.initialID, svc.ID)
			}
		})
	}
}

func TestLabels_Nil(t *testing.T) {
	var labels Labels

	assert.Nil(t, labels)
}

func TestLabels_Empty(t *testing.T) {
	labels := Labels{}

	assert.NotNil(t, labels)
	assert.Empty(t, labels)
}

func TestLabels_WithValues(t *testing.T) {
	labels := Labels{
		"key1": "value1",
		"key2": "value2",
	}

	assert.Equal(t, "value1", labels["key1"])
	assert.Equal(t, "value2", labels["key2"])
	assert.Len(t, labels, 2)
}

func TestService_Fields(t *testing.T) {
	id := uuid.New()
	svc := Service{
		ID:          id,
		Name:        "test-service",
		Description: "Test description",
		Environment: "dev",
		Labels:      Labels{"key": "value"},
		DeletedAt:   gorm.DeletedAt{Valid: false},
	}

	assert.Equal(t, id, svc.ID)
	assert.Equal(t, "test-service", svc.Name)
	assert.Equal(t, "Test description", svc.Description)
	assert.Equal(t, "dev", svc.Environment)
	assert.Equal(t, Labels{"key": "value"}, svc.Labels)
}
