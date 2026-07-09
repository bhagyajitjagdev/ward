//go:build embed_ui

package web

import (
	"embed"
	"io/fs"
)

// dist holds the built ward-ui (copied to internal/web/dist by the Docker build
// before `go build -tags embed_ui`).
//
//go:embed all:dist
var dist embed.FS

// Assets returns the embedded UI filesystem; ok is true when the UI was compiled in.
func Assets() (fs.FS, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}
	return sub, true
}
