// Package model holds the domain/API types (transport-facing, no DB tags).
package model

import "time"

// Service is a proxied backend Ward fronts. Upstreams is the list of dial
// targets Caddy load-balances across.
type Service struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	PublicHostname string    `json:"public_hostname"`
	Upstreams      []string  `json:"upstreams"`
	LBPolicy       string    `json:"lb_policy"`
	TLSMode        string    `json:"tls_mode"`
	WAFEnabled     bool      `json:"waf_enabled"`
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
