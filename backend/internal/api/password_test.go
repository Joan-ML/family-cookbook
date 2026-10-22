package api

import (
	"net/http"
	"strings"
	"testing"
)

func setupJSON(token, password string) []byte {
	return []byte(`{"token":"` + token + `","password":"` + password + `"}`)
}

func changeJSON(current, next string) []byte {
	return []byte(`{"currentPassword":"` + current + `","newPassword":"` + next + `"}`)
}

func TestSetupLinkSetsThePasswordAndLogsIn(t *testing.T) {
	srv, s := newTestServer(t)
	if err := srv.db.CreateUser(t.Context(), "anna", "", testNow); err != nil {
		t.Fatal(err)
	}
	token, _, err := s.auth.IssueSetupLink(t.Context(), "anna")
	if err != nil {
		t.Fatal(err)
	}
	anna := srv.anonymous()

	res := anna.do(t, http.MethodPost, "/api/password/setup", setupJSON(token, "annas new password"))

	wantStatus(t, res, http.StatusOK)
	if u := decode[User](t, res); u.Username != "anna" {
		t.Errorf("logged in as %q", u.Username)
	}
	if !strings.Contains(res.Header.Get("Set-Cookie"), "session=") {
		t.Fatal("no session cookie")
	}
	anna.cookies = res.Cookies()
	wantStatus(t, anna.do(t, http.MethodGet, "/api/me", nil), http.StatusOK)

	// The link works once.
	res = srv.anonymous().do(t, http.MethodPost, "/api/password/setup", setupJSON(token, "someone elses password"))
	wantStatus(t, res, http.StatusBadRequest)
	if msg := decode[Error](t, res).Message; !strings.Contains(msg, "invalid, already used or expired") {
		t.Errorf("message %q", msg)
	}
}

func TestSetupLinkRejectsWeakPasswordsAndBadTokens(t *testing.T) {
	srv, s := newTestServer(t)
	token, _, err := s.auth.IssueSetupLink(t.Context(), testUser)
	if err != nil {
		t.Fatal(err)
	}
	anon := srv.anonymous()

	res := anon.do(t, http.MethodPost, "/api/password/setup", setupJSON(token, "short"))
	wantStatus(t, res, http.StatusBadRequest)
	if msg := decode[Error](t, res).Message; msg != "passwords need at least 10 characters" {
		t.Errorf("message %q", msg)
	}
	wantStatus(t, anon.do(t, http.MethodPost, "/api/password/setup", setupJSON("made-up", "a long enough password")), http.StatusBadRequest)

	// A weak attempt doesn't use up the link.
	wantStatus(t, anon.do(t, http.MethodPost, "/api/password/setup", setupJSON(token, "a long enough password")), http.StatusOK)
}

func TestChangePassword(t *testing.T) {
	srv, _ := newTestServer(t)
	other := srv.anonymous()
	other.logIn(t, testUser, testPassword) // e.g. on another device

	res := srv.do(t, http.MethodPut, "/api/me/password", changeJSON("wrong", "a brand new password"))
	// 400, not 401: a 401 would make the app think the session ended.
	wantStatus(t, res, http.StatusBadRequest)
	if msg := decode[Error](t, res).Message; msg != "the current password is wrong" {
		t.Errorf("message %q", msg)
	}
	wantStatus(t, srv.do(t, http.MethodPut, "/api/me/password", changeJSON(testPassword, "short")), http.StatusBadRequest)

	wantStatus(t, srv.do(t, http.MethodPut, "/api/me/password", changeJSON(testPassword, "a brand new password")), http.StatusNoContent)

	wantStatus(t, srv.do(t, http.MethodGet, "/api/me", nil), http.StatusOK)             // this session stays
	wantStatus(t, other.do(t, http.MethodGet, "/api/me", nil), http.StatusUnauthorized) // others end
	fresh := srv.anonymous()
	fresh.logIn(t, testUser, "a brand new password")

	wantStatus(t, srv.anonymous().do(t, http.MethodPut, "/api/me/password", changeJSON("x", "a brand new password")), http.StatusUnauthorized)
}

func TestChangePasswordIsRateLimited(t *testing.T) {
	srv, _ := newTestServer(t)
	for range 5 {
		srv.do(t, http.MethodPut, "/api/me/password", changeJSON("guess", "a brand new password"))
	}

	res := srv.do(t, http.MethodPut, "/api/me/password", changeJSON(testPassword, "a brand new password"))

	wantStatus(t, res, http.StatusTooManyRequests)
	if res.Header.Get("Retry-After") == "" {
		t.Error("no Retry-After")
	}
}
