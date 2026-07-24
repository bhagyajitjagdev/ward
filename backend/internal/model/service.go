// Package model holds the domain/API types (transport-facing, no DB tags).
package model

import "time"

// Service is a proxied backend Ward fronts. Upstreams is the list of dial
// targets Caddy load-balances across.
type Service struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// PublicHostname is the primary hostname (first of PublicHostnames) — kept as a
	// read-only alias for a gentle transition; input may send it or PublicHostnames.
	PublicHostname string `json:"public_hostname"`
	// PublicHostnames is every hostname this service answers on. One route, one policy.
	PublicHostnames []string   `json:"public_hostnames"`
	Upstreams       []string   `json:"upstreams"`
	LBPolicy        string     `json:"lb_policy"`
	TLSMode         string     `json:"tls_mode"`
	WAFEnabled      bool       `json:"waf_enabled"`
	WAFMode         string     `json:"waf_mode"` // "" = inherit global default | "DetectionOnly" | "On"
	// WAFSkipPaths are request paths (matched as prefix + subpaths) for which the WAF
	// is bypassed so streaming works — the Coraza handler buffers responses whenever
	// it's in the path. WebSocket upgrades bypass automatically, independent of this.
	WAFSkipPaths []string   `json:"waf_skip_paths"`
	HTTP         HTTPConfig `json:"http"`                // structured proxy controls (headers, auth, rewrite…)
	RawCaddy        string     `json:"raw_caddy,omitempty"` // advanced escape hatch: a Caddyfile fragment
	Enabled         bool       `json:"enabled"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// HTTPConfig holds a service's structured HTTP/proxy controls, rendered into its
// generated route. Stored as a JSON blob on the service row.
type HTTPConfig struct {
	SecurityHeaders bool              `json:"security_headers,omitempty"` // one-click HSTS + safe response headers
	RequestHeaders  map[string]string `json:"request_headers,omitempty"`  // set on the request to the upstream
	ResponseHeaders map[string]string `json:"response_headers,omitempty"` // set on the response to the client
	RemoveHeaders   []string          `json:"remove_headers,omitempty"`   // response headers to strip (e.g. Server)
	BasicAuthUser   string            `json:"basic_auth_user,omitempty"`
	// BasicAuthHash is the bcrypt hash at rest. Stored in the DB blob but blanked by
	// the API before a service leaves the process — never returned to a client.
	BasicAuthHash string `json:"basic_auth_hash,omitempty"`
	// BasicAuthPassword is write-only input: the API bcrypts it into BasicAuthHash and
	// never stores or returns it.
	BasicAuthPassword string `json:"basic_auth_password,omitempty"`
	StripPathPrefix   string `json:"strip_path_prefix,omitempty"`
	Compression       bool   `json:"compression,omitempty"`
}
