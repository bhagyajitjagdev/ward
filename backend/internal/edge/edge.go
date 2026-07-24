// Package edge records the versions of the components compiled into the ward-caddy
// image this Ward release is built to run against. Ground truth for a *running* image
// is its OCI labels (docker inspect); these values are the human-friendly copy shown
// in the UI, kept in lockstep with caddy/Dockerfile by a unit test (versions_test.go).
package edge

import (
	_ "embed"
	"encoding/json"
)

//go:embed versions.json
var versionsJSON []byte

// Versions returns the pinned edge component versions as component → version, e.g.
// {"caddy": "v2.11.4", "crs": "v4.25.0", …}.
func Versions() map[string]string {
	var m map[string]string
	_ = json.Unmarshal(versionsJSON, &m)
	return m
}
