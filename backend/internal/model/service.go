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
	WAFSkipPaths []string    `json:"waf_skip_paths"`
	HTTP         HTTPConfig  `json:"http"`         // structured proxy controls (headers, auth, rewrite…)
	HealthCheck  HealthCheck `json:"health_check"` // active upstream health checking (opt-in)
	// Redirect, when To is set, makes this a redirect-only service (301/302) instead of
	// a proxy — no upstreams needed, and the WAF/HTTP-controls chain is skipped.
	Redirect Redirect `json:"redirect"`
	RawCaddy string   `json:"raw_caddy,omitempty"` // advanced escape hatch: a Caddyfile fragment
	Enabled  bool     `json:"enabled"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Redirect turns a service into a redirect (301/302) instead of a proxy. To is the
// target URL; when PreservePath is set the original path+query are appended.
type Redirect struct {
	To           string `json:"to,omitempty"`            // target URL, e.g. https://new.example.com
	Status       int    `json:"status,omitempty"`        // 301 or 302; 0 → 302
	PreservePath bool   `json:"preserve_path,omitempty"` // append the original path + query
}

// HealthCheck holds a service's active upstream health-check config (opt-in). Passive
// checks are always on (a failing upstream is pulled from rotation); active checks add
// a periodic probe. Stored as a JSON blob on the service row.
type HealthCheck struct {
	Active       bool   `json:"active,omitempty"`        // enable active probing
	Path         string `json:"path,omitempty"`          // probe URI (default "/")
	Interval     string `json:"interval,omitempty"`      // Go duration, e.g. "10s"
	Timeout      string `json:"timeout,omitempty"`       // Go duration, e.g. "5s"
	ExpectStatus int    `json:"expect_status,omitempty"` // healthy status code; 0 → Caddy default (2xx)
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
