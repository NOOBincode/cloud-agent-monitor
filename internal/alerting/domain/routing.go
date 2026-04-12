package domain

import (
	"time"

	"github.com/google/uuid"
)

type Route struct {
	ID              uuid.UUID       `json:"id"`
	Name            string          `json:"name"`
	Receiver        string          `json:"receiver"`
	GroupBy         []string        `json:"group_by"`
	GroupWait       time.Duration   `json:"group_wait"`
	GroupInterval   time.Duration   `json:"group_interval"`
	RepeatInterval  time.Duration   `json:"repeat_interval"`
	Match           map[string]string `json:"match"`
	MatchRE         map[string]string `json:"match_re"`
	Continue        bool            `json:"continue"`
	Routes          []*Route        `json:"routes,omitempty"`
	Active          bool            `json:"active"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

type RoutingConfig struct {
	Global Route `json:"global"`
	Routes []*Route `json:"routes"`
}

func NewRoute(name, receiver string) *Route {
	now := time.Now()
	return &Route{
		ID:             uuid.New(),
		Name:           name,
		Receiver:       receiver,
		GroupBy:        []string{"alertname", "severity"},
		GroupWait:      30 * time.Second,
		GroupInterval:  5 * time.Minute,
		RepeatInterval: 4 * time.Hour,
		Match:          make(map[string]string),
		MatchRE:        make(map[string]string),
		Active:         true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func (r *Route) AddMatch(key, value string) {
	if r.Match == nil {
		r.Match = make(map[string]string)
	}
	r.Match[key] = value
}

func (r *Route) AddMatchRE(key, pattern string) {
	if r.MatchRE == nil {
		r.MatchRE = make(map[string]string)
	}
	r.MatchRE[key] = pattern
}

func (r *Route) Matches(labels map[string]string) bool {
	for key, value := range r.Match {
		if labels[key] != value {
			return false
		}
	}
	return true
}

type RouteFilter struct {
	Receiver string `form:"receiver"`
	Active   *bool  `form:"active"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
