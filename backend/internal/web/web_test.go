package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestSPAHandler(t *testing.T) {
	dist := fstest.MapFS{
		"index.html":         {Data: []byte("app shell")},
		"favicon.svg":        {Data: []byte("<svg/>")},
		"assets/index-a1.js": {Data: []byte("js")},
	}
	h := spaHandler(dist)

	tests := []struct {
		path      string
		wantBody  string
		wantCache string
	}{
		{"/", "app shell", "no-cache"},
		{"/recipes/123", "app shell", "no-cache"}, // client-side route
		{"/assets", "app shell", "no-cache"},      // directories aren't listed
		{"/favicon.svg", "<svg/>", ""},
		{"/assets/index-a1.js", "js", "public, max-age=31536000, immutable"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))

			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if got := rec.Body.String(); got != tt.wantBody {
				t.Errorf("body = %q, want %q", got, tt.wantBody)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.wantCache {
				t.Errorf("Cache-Control = %q, want %q", got, tt.wantCache)
			}
		})
	}
}
