package model

import "time"

// AccessEvent is one HTTP request as recorded by Caddy's access log. Short-lived
// (retention in days) — Ward keeps it for the dashboard + a recent-requests view;
// the long-term store is an external log pipeline.
type AccessEvent struct {
	ID         string    `json:"id"`
	TS         time.Time `json:"ts"`
	ServiceID  *string   `json:"service_id,omitempty"`
	Host       string    `json:"host"`
	ClientIP   string    `json:"client_ip"`
	Method     string    `json:"method"`
	Path       string    `json:"path"`
	Query      string    `json:"query,omitempty"`
	Status     int       `json:"status"`
	DurationMs float64   `json:"duration_ms"`
	Bytes      int64     `json:"bytes"`
	UserAgent  string    `json:"user_agent,omitempty"`
}
