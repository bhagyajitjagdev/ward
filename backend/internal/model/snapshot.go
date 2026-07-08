package model

import "time"

// ConfigSnapshot is an applied Caddy config, kept for rollback. CaddyJSON is
// omitted from list responses (it's large) and only populated on fetch.
type ConfigSnapshot struct {
	ID        string    `json:"id"`
	Note      string    `json:"note,omitempty"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	CaddyJSON string    `json:"caddy_json,omitempty"`
}
