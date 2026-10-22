package api

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"family-cookbook/backend/internal/auth"
)

func loginBody(username, password string) []byte {
	return []byte(`{"username":"` + username + `","password":"` + password + `"}`)
}

func TestLogInSetsASessionCookie(t *testing.T) {
	srv, _ := newTestServer(t)
	anon := srv.anonymous()

	res := anon.do(t, http.MethodPost, "/api/session", loginBody("DEV", testPassword))

	wantStatus(t, res, http.StatusOK)
	if got := decode[User](t, res).Username; got != testUser {
		t.Errorf("username = %q, want %q (normalized)", got, testUser)
	}
	cookie := res.Header.Get("Set-Cookie")
	for _, want := range []string{"session=", "Path=/", "HttpOnly", "SameSite=Lax", "Max-Age=2592000"} {
		if !strings.Contains(cookie, want) {
			t.Errorf("Set-Cookie %q lacks %q", cookie, want)
		}
	}
	// The test server is on 127.0.0.1, so the cookie isn't marked Secure.
	if strings.Contains(cookie, "Secure") {
		t.Errorf("Set-Cookie %q is Secure on localhost", cookie)
	}

	anon.cookies = res.Cookies()
	me := anon.do(t, http.MethodGet, "/api/me", nil)
	wantStatus(t, me, http.StatusOK)
	if got := decode[User](t, me).Username; got != testUser {
		t.Errorf("/me = %q", got)
	}
}

func TestSessionCookieIsSecureExceptOnLocalhost(t *testing.T) {
	_, s := newTestServer(t)
	h := newHandler(s)

	tests := []struct {
		host     string
		insecure bool
		want     bool
	}{
		{"cookbook.example.com", false, true},
		{"cookbook.example.com:443", false, true},
		{"localhost:8080", false, false},
		{"127.0.0.1:8080", false, false},
		{"[::1]:8080", false, false},
		{"app.localhost", false, false},
		{"192.168.1.20:8080", true, false}, // INSECURE_COOKIES for plain HTTP on a LAN
	}
	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			s.insecureCookies = tt.insecure
			req := httptest.NewRequest(http.MethodPost, "/api/session",
				strings.NewReader(string(loginBody(testUser, testPassword))))
			req.Host = tt.host
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("status %d: %s", rec.Code, rec.Body)
			}
			if got := strings.Contains(rec.Header().Get("Set-Cookie"), "Secure"); got != tt.want {
				t.Errorf("Secure = %v, want %v (Set-Cookie %q)", got, tt.want, rec.Header().Get("Set-Cookie"))
			}
		})
	}
}

func TestLogInWithBadCredentials(t *testing.T) {
	srv, _ := newTestServer(t)
	anon := srv.anonymous()

	wrongPassword := anon.do(t, http.MethodPost, "/api/session", loginBody(testUser, "wrong password"))
	unknownUser := anon.do(t, http.MethodPost, "/api/session", loginBody("nobody", testPassword))

	for _, res := range []*http.Response{wrongPassword, unknownUser} {
		wantStatus(t, res, http.StatusUnauthorized)
		if res.Header.Get("Set-Cookie") != "" {
			t.Error("a failed login set a cookie")
		}
		// Same message either way, so usernames can't be probed.
		if got := decode[Error](t, res).Message; got != "wrong username or password" {
			t.Errorf("message = %q", got)
		}
	}
}

func TestRepeatedFailedLogInsAreRefusedForAWhile(t *testing.T) {
	srv, _ := newTestServer(t)
	anon := srv.anonymous()
	for range 5 {
		anon.do(t, http.MethodPost, "/api/session", loginBody(testUser, "wrong password"))
	}

	res := anon.do(t, http.MethodPost, "/api/session", loginBody(testUser, testPassword))

	wantStatus(t, res, http.StatusTooManyRequests)
	if secs, err := strconv.Atoi(res.Header.Get("Retry-After")); err != nil || secs <= 0 || secs > 15*60 {
		t.Errorf("Retry-After = %q, want seconds up to 15 minutes", res.Header.Get("Retry-After"))
	}
}

func TestEndpointsNeedASessionUnlessPublic(t *testing.T) {
	srv, _ := newTestServer(t)
	anon := srv.anonymous()

	for _, path := range []string{"/api/cookbooks", srv.book, srv.book + "/recipes", srv.book + "/recipes/r1", "/api/me"} {
		res := anon.do(t, http.MethodGet, path, nil)
		wantStatus(t, res, http.StatusUnauthorized)
		if got := decode[Error](t, res).Message; got != "not logged in" {
			t.Errorf("%s: message = %q", path, got)
		}
	}
	res := anon.do(t, http.MethodPost, srv.book+"/recipes", mustJSON(t, pancakes()))
	wantStatus(t, res, http.StatusUnauthorized)

	for _, path := range []string{"/api/health", "/api/info"} {
		wantStatus(t, anon.do(t, http.MethodGet, path, nil), http.StatusOK)
	}
}

func TestLogOut(t *testing.T) {
	srv, _ := newTestServer(t)
	session := srv.cookies

	res := srv.do(t, http.MethodDelete, "/api/session", nil)

	wantStatus(t, res, http.StatusNoContent)
	if c := res.Header.Get("Set-Cookie"); !strings.Contains(c, "session=;") || !strings.Contains(c, "Max-Age=0") {
		t.Errorf("Set-Cookie %q doesn't clear the session cookie", c)
	}
	// The old cookie no longer works, even if a client keeps sending it.
	stale := srv.anonymous()
	stale.cookies = session
	wantStatus(t, stale.do(t, http.MethodGet, "/api/me", nil), http.StatusUnauthorized)
	// Logging out again, or without a session, still succeeds.
	wantStatus(t, stale.do(t, http.MethodDelete, "/api/session", nil), http.StatusNoContent)
	wantStatus(t, srv.anonymous().do(t, http.MethodDelete, "/api/session", nil), http.StatusNoContent)
}

func TestSessionsExpireAndRenew(t *testing.T) {
	srv, s := newTestServer(t)

	// Used after a couple of days, the session is extended and the cookie re-sent.
	tick(s, 48*time.Hour)
	res := srv.do(t, http.MethodGet, "/api/me", nil)
	wantStatus(t, res, http.StatusOK)
	if !strings.Contains(res.Header.Get("Set-Cookie"), "Max-Age=2592000") {
		t.Errorf("renewed session didn't refresh the cookie: %q", res.Header.Get("Set-Cookie"))
	}

	// Left alone for the whole session lifetime, it ends.
	tick(s, auth.SessionTTL)
	wantStatus(t, srv.do(t, http.MethodGet, "/api/me", nil), http.StatusUnauthorized)
}

func TestRecipesRecordWhoEditedThem(t *testing.T) {
	srv, _ := newTestServer(t)
	created := createRecipe(t, srv, pancakes())

	if created.CreatedBy != testUser || created.UpdatedBy != testUser {
		t.Errorf("createdBy %q, updatedBy %q; want %q", created.CreatedBy, created.UpdatedBy, testUser)
	}
}
