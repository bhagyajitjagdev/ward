package model

import "time"

// ConfigSnapshot is an applied Caddy config, kept for rollback. CaddyJSON is
// omitted from list responses (it's large) and only populated on fetch.
type ConfigSnapshot struct {
	ID   string `json:"id"`
	Note string `json:"note,omitempty"`
	// Active marks the snapshot the edge is currently running.
	Active bool `json:"active"`
	// Restorable reports whether the snapshot carries Ward's declarative state, so a
	// rollback can restore the DB from it. Snapshots taken before that state was
	// recorded hold only the rendered Caddy JSON and can't be rolled back to.
	Restorable bool      `json:"restorable"`
	CreatedAt  time.Time `json:"created_at"`
	CaddyJSON  string    `json:"caddy_json,omitempty"`
	// WardJSON is the desired state (store.DesiredState) the config was generated
	// from. Internal only: it carries basic-auth hashes, so it never leaves the API.
	WardJSON string `json:"-"`
}
