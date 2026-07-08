package model

import "time"

// RateLimit is a per-IP request cap, scoped globally (whole edge) or to one
// service — symmetric with BlockedIP. Enforced at the edge by the caddy-ratelimit
// module, keyed by client IP; requests over the cap get a 429.
type RateLimit struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"` // "global" | "service"
	ServiceID *string   `json:"service_id,omitempty"`
	MaxEvents int       `json:"max_events"` // requests allowed per window
	Window    string    `json:"window"`     // duration, e.g. "1m", "10s", "1h"
	CreatedAt time.Time `json:"created_at"`
}
