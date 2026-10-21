// Package web serves the built frontend.
//
// The frontend is only embedded when building with `-tags embedui` (as the
// Dockerfile does, after copying frontend/dist into ./dist). Without the tag,
// Handler returns nil and the frontend is served by Vite during development.
package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// spaHandler serves files from dist, and index.html for any other path so
// the client-side router can handle URLs like /recipes/123.
func spaHandler(dist fs.FS) http.Handler {
	files := http.FileServerFS(dist)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || !exists(dist, name) {
			// The app shell must never be cached, so new releases load at once.
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeFileFS(w, r, dist, "index.html")
			return
		}
		if strings.HasPrefix(name, "assets/") {
			// Vite puts a content hash in these file names.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		files.ServeHTTP(w, r)
	})
}

func exists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
