//go:build embedui

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var embedded embed.FS

// Handler serves the embedded frontend.
func Handler() http.Handler {
	dist, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // the embed path is fixed, so this is a build bug
	}
	return spaHandler(dist)
}
