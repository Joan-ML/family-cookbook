//go:build !embedui

package web

import "net/http"

// Handler returns nil: this binary was built without the frontend.
func Handler() http.Handler {
	return nil
}
