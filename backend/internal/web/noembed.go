//go:build !embed_ui

package web

import "io/fs"

// Assets reports that no UI was compiled in (plain `go build`, no embed_ui tag).
func Assets() (fs.FS, bool) { return nil, false }
