package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"family-cookbook/backend/internal/store/sqlite"
	"family-cookbook/backend/internal/version"
)

// testNow is the fixed clock used by test servers; tests advance it with tick.
var testNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// newTestServer returns a contract-checked server backed by a fresh SQLite
// database, with a controllable clock and predictable IDs.
func newTestServer(t *testing.T) (*contractServer, *server) {
	t.Helper()

	st, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })

	now := testNow
	ids := 0
	s := &server{
		store: st,
		now:   func() time.Time { return now },
		newID: func() string { ids++; return fmt.Sprintf("r%d", ids) },
	}
	return newContractServer(t, newHandler(s)), s
}

// tick advances a test server's clock.
func tick(s *server, d time.Duration) {
	next := s.now().Add(d)
	s.now = func() time.Time { return next }
}

// decode reads a JSON response body into a value of type T.
func decode[T any](t *testing.T, res *http.Response) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(res.Body).Decode(&v); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	return v
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func wantStatus(t *testing.T, res *http.Response, want int) {
	t.Helper()
	if res.StatusCode != want {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status = %d, want %d; body: %s", res.StatusCode, want, body)
	}
}

func TestHealth(t *testing.T) {
	srv, _ := newTestServer(t)

	res := srv.do(t, http.MethodGet, "/api/health", nil)

	wantStatus(t, res, http.StatusOK)
	if got := decode[Health](t, res).Version; got != version.Version {
		t.Errorf("version = %q, want %q", got, version.Version)
	}
}

func TestOversizedBodyReturns413(t *testing.T) {
	srv, _ := newTestServer(t)
	in := pancakes()
	in.Steps = []string{strings.Repeat("a", maxBodyBytes)}

	res := srv.doInvalid(t, http.MethodPost, "/api/recipes", mustJSON(t, in))

	wantStatus(t, res, http.StatusRequestEntityTooLarge)
}

func TestUnknownRouteReturnsJSON404(t *testing.T) {
	srv, _ := newTestServer(t)

	res, err := srv.ts.Client().Get(srv.ts.URL + "/api/nope")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	wantStatus(t, res, http.StatusNotFound)
	if e := decode[Error](t, res); e.Message == "" {
		t.Error("expected an error message")
	}
}
