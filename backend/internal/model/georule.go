package model

import "time"

// GeoRule blocks requests from a set of countries, scoped globally (whole edge)
// or to one service — symmetric with BlockedIP. Enforced at the edge by the
// caddy-maxmind-geolocation matcher (requires a GeoIP database); blocked → 403.
type GeoRule struct {
	ID        string    `json:"id"`
	Scope     string    `json:"scope"` // "global" | "service"
	ServiceID *string   `json:"service_id,omitempty"`
	Countries []string  `json:"countries"` // ISO 3166-1 alpha-2 codes, e.g. ["RU","CN"]
	CreatedAt time.Time `json:"created_at"`
}
