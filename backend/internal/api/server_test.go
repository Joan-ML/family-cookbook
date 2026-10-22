package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"family-cookbook/backend/internal/auth"
	"family-cookbook/backend/internal/store/sqlite"
)

// testNow is the fixed clock used by test servers; tests advance it with tick.
var testNow = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// The user every test server has, and is logged in as.
const (
	testUser     = "dev"
	testPassword = "test password 123"
)

// testUserHash is computed once: argon2id is deliberately slow.
var testUserHash = sync.OnceValue(func() string {
	h, err := auth.HashPassword(testPassword)
	if err != nil {
		panic(err)
	}
	return h
})

// newTestServer returns a contract-checked server backed by a fresh SQLite
// database, with a controllable clock and predictable IDs, logged in as
// testUser. Use srv.anonymous() for requests without a session.
func newTestServer(t *testing.T) (*contractServer, *server) {
	t.Helper()

	st, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if err := st.CreateUser(t.Context(), testUser, testUserHash(), testNow); err != nil {
		t.Fatal(err)
	}

	now := testNow
	ids := 0
	s := &server{
		store: st,
		now:   func() time.Time { return now },
		newID: func() string { ids++; return fmt.Sprintf("r%d", ids) },
	}
	// Sessions follow the same clock, which tick moves.
	s.auth = auth.NewService(st, func() time.Time { return s.now() })

	srv := newContractServer(t, newHandler(s))
	srv.db = st
	srv.logIn(t, testUser, testPassword)
	srv.book = srv.newCookbook(t, "Test kitchen")
	return srv, s
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
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name, release string
		want          Info
	}{
		{"released", "v1.2.3", Info{Commit: "abc1234", Version: ptr("v1.2.3")}},
		{"unreleased", "", Info{Commit: "abc1234"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, s := newTestServer(t)
			s.release, s.commit = tt.release, "abc1234"

			res := srv.do(t, http.MethodGet, "/api/info", nil)

			wantStatus(t, res, http.StatusOK)
			got := decode[Info](t, res)
			if got.Commit != tt.want.Commit || !equalPtr(got.Version, tt.want.Version) {
				t.Errorf("info = %s, want %s", mustJSON(t, got), mustJSON(t, tt.want))
			}
		})
	}
}

func TestOversizedBodyReturns413(t *testing.T) {
	srv, _ := newTestServer(t)
	in := pancakes()
	in.Steps = []string{strings.Repeat("a", maxBodyBytes)}

	res := srv.doInvalid(t, http.MethodPost, srv.book+"/recipes", mustJSON(t, in))

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
