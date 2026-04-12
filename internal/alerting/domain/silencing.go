package domain

import (
	"time"

	"github.com/google/uuid"
)

type SilenceStatus string

const (
	SilenceStatusActive   SilenceStatus = "active"
	SilenceStatusExpired  SilenceStatus = "expired"
	SilenceStatusPending  SilenceStatus = "pending"
)

type Matcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"is_regex"`
}

type Silence struct {
	ID        uuid.UUID     `json:"id"`
	Matchers  []Matcher     `json:"matchers"`
	StartsAt  time.Time     `json:"starts_at"`
	EndsAt    time.Time     `json:"ends_at"`
	CreatedBy string        `json:"created_by"`
	Comment   string        `json:"comment"`
	Status    SilenceStatus `json:"status"`
	CreatedAt time.Time     `json:"created_at"`
}

func NewSilence(matchers []Matcher, startsAt, endsAt time.Time, createdBy, comment string) *Silence {
	now := time.Now()
	status := SilenceStatusPending
	if now.After(startsAt) {
		status = SilenceStatusActive
	}

	return &Silence{
		ID:        uuid.New(),
		Matchers:  matchers,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		CreatedBy: createdBy,
		Comment:   comment,
		Status:    status,
		CreatedAt: now,
	}
}

func (s *Silence) IsActive() bool {
	now := time.Now()
	return now.After(s.StartsAt) && now.Before(s.EndsAt)
}

func (s *Silence) IsExpired() bool {
	return time.Now().After(s.EndsAt)
}

func (s *Silence) Matches(labels map[string]string) bool {
	for _, m := range s.Matchers {
		value, exists := labels[m.Name]
		if !exists {
			return false
		}
		if m.IsRegex {
			return true
		}
		if value != m.Value {
			return false
		}
	}
	return true
}

func (s *Silence) UpdateStatus() {
	now := time.Now()
	if now.Before(s.StartsAt) {
		s.Status = SilenceStatusPending
	} else if now.After(s.EndsAt) {
		s.Status = SilenceStatusExpired
	} else {
		s.Status = SilenceStatusActive
	}
}

type SilenceFilter struct {
	Active    *bool      `form:"active"`
	CreatedBy string     `form:"created_by"`
	Status    SilenceStatus `form:"status"`
	Page      int        `form:"page"`
	PageSize  int        `form:"page_size"`
}
