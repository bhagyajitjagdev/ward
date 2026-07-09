// Package web serves the embedded ward-ui single-page app. The UI is compiled in
// only under the `embed_ui` build tag (the release image); a plain `go build`
// leaves it out, so backend-only development doesn't need the built assets.
package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// SPAHandler serves static assets from fsys and falls back to index.html for
// client-side routes (any path that isn't a real file), so deep links work.
func SPAHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if p != "" {
			if f, err := fsys.Open(p); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r) // a real asset (js/css/img) — content-typed + cached by FileServer
				return
			}
		}
		serveIndex(w, fsys) // "/" or an unknown route → the SPA shell
	})
}

func serveIndex(w http.ResponseWriter, fsys fs.FS) {
	b, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		http.Error(w, "ui not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}
