package auth

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"family-cookbook/backend/internal/store/sqlite"
)

const password = "correct horse battery"

type clock struct{ t time.Time }

func (c *clock) now() time.Time          { return c.t }
func (c *clock) advance(d time.Duration) { c.t = c.t.Add(d) }

// newService returns a Service on a fresh database with user "joan".
func newService(t *testing.T) (*Service, *sqlite.Store, *clock) {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	hash, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	c := &clock{t: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)}
	if err := st.CreateUser(context.Background(), "joan", hash, c.now()); err != nil {
		t.Fatal(err)
	}
	return NewService(st, st, c.now), st, c
}

func TestPasswordHashing(t *testing.T) {
	h1, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	h2, _ := HashPassword(password)
	if !strings.HasPrefix(h1, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("unexpected hash format %q", h1)
	}
	if h1 == h2 {
		t.Error("two hashes of the same password are equal: salt missing")
	}
	if ok, err := VerifyPassword(h1, password); !ok || err != nil {
		t.Errorf("correct password rejected (err %v)", err)
	}
	if ok, _ := VerifyPassword(h1, password+"x"); ok {
		t.Error("wrong password accepted")
	}
	if _, err := VerifyPassword("$2a$10$bcrypt", password); err == nil {
		t.Error("unknown hash format accepted")
	}
}

func TestNormalizeUsername(t *testing.T) {
	for in, want := range map[string]string{"Joan": "joan", " anna.m_1-x ": "anna.m_1-x"} {
		if got, err := NormalizeUsername(in); got != want || err != nil {
			t.Errorf("NormalizeUsername(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "has space", "joan!", "émile", strings.Repeat("a", 65)} {
		if _, err := NormalizeUsername(bad); err == nil {
			t.Errorf("NormalizeUsername(%q) accepted", bad)
		}
	}
}

func TestCheckNewPassword(t *testing.T) {
	if err := CheckNewPassword("short"); err == nil {
		t.Error("short password accepted")
	}
	if err := CheckNewPassword("àèìòùàèìòù"); err != nil {
		t.Errorf("10 non-ASCII characters rejected: %v", err)
	}
}

func TestLogInAndAuthenticate(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()

	token, sess, err := s.LogIn(ctx, "Joan", password) // usernames are case-insensitive
	if err != nil {
		t.Fatal(err)
	}
	if sess.Username != "joan" || token == "" || sess.TokenHash == token {
		t.Fatalf("unexpected session %+v (token %q)", sess, token)
	}
	got, _, err := s.Authenticate(ctx, token)
	if err != nil || got.Username != "joan" {
		t.Fatalf("Authenticate = %+v, %v", got, err)
	}
	if _, _, err := s.Authenticate(ctx, token+"x"); !errors.Is(err, ErrNoSession) {
		t.Errorf("tampered token: err = %v, want ErrNoSession", err)
	}
}

func TestLogInRejectsBadCredentialsTheSameWay(t *testing.T) {
	s, _, _ := newService(t)
	for _, tt := range []struct{ user, pass string }{
		{"joan", "wrong password"},
		{"nobody", password},
		{"not a valid name!", password},
	} {
		if _, _, err := s.LogIn(context.Background(), tt.user, tt.pass); !errors.Is(err, ErrInvalidCredentials) {
			t.Errorf("LogIn(%q, %q) err = %v, want ErrInvalidCredentials", tt.user, tt.pass, err)
		}
	}
}

func TestRepeatedFailuresLockTheUsernameForAWhile(t *testing.T) {
	s, _, c := newService(t)
	ctx := context.Background()
	for range maxFailures {
		if _, _, err := s.LogIn(ctx, "joan", "wrong password"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("err = %v", err)
		}
	}

	// Locked: even the right password is refused.
	_, _, err := s.LogIn(ctx, "joan", password)
	var tooMany *TooManyAttemptsError
	if !errors.As(err, &tooMany) || tooMany.RetryAfter != failureWindow {
		t.Fatalf("err = %v, want TooManyAttemptsError with RetryAfter %s", err, failureWindow)
	}

	c.advance(failureWindow)
	if _, _, err := s.LogIn(ctx, "joan", password); err != nil {
		t.Fatalf("still locked after the window: %v", err)
	}
}

func TestSuccessfulLogInResetsFailures(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	for range maxFailures - 1 {
		_, _, _ = s.LogIn(ctx, "joan", "wrong password")
	}
	if _, _, err := s.LogIn(ctx, "joan", password); err != nil {
		t.Fatal(err)
	}
	for range maxFailures - 1 {
		_, _, _ = s.LogIn(ctx, "joan", "wrong password")
	}
	if _, _, err := s.LogIn(ctx, "joan", password); err != nil {
		t.Errorf("locked out although failures were reset: %v", err)
	}
}

func TestSessionsExpireUnlessUsed(t *testing.T) {
	s, _, c := newService(t)
	ctx := context.Background()
	token, _, err := s.LogIn(ctx, "joan", password)
	if err != nil {
		t.Fatal(err)
	}

	c.advance(time.Hour)
	if _, renewed, err := s.Authenticate(ctx, token); err != nil || renewed {
		t.Errorf("after an hour: renewed=%v err=%v; want no renewal yet", renewed, err)
	}

	// Used now and then, a session lives on past its first 30 days.
	for range 3 {
		c.advance(20 * 24 * time.Hour)
		sess, renewed, err := s.Authenticate(ctx, token)
		if err != nil || !renewed || !sess.ExpiresAt.Equal(c.now().Add(SessionTTL)) {
			t.Fatalf("after 20 days: %+v renewed=%v err=%v", sess, renewed, err)
		}
	}

	// Left unused for 30 days, it ends.
	c.advance(SessionTTL)
	if _, _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("unused for 30 days: err = %v, want ErrNoSession", err)
	}
}

func TestLogOut(t *testing.T) {
	s, _, _ := newService(t)
	ctx := context.Background()
	token, _, err := s.LogIn(ctx, "joan", password)
	if err != nil {
		t.Fatal(err)
	}

	if err := s.LogOut(ctx, token); err != nil {
		t.Fatal(err)
	}

	if _, _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("err = %v, want ErrNoSession", err)
	}
	if err := s.LogOut(ctx, token); err != nil {
		t.Errorf("logging out twice: %v", err)
	}
}

func TestPasswordResetAndRemovalEndSessions(t *testing.T) {
	s, st, _ := newService(t)
	ctx := context.Background()
	token, _, err := s.LogIn(ctx, "joan", password)
	if err != nil {
		t.Fatal(err)
	}
	hash, _ := HashPassword("a brand new password")
	if err := st.SetPassword(ctx, "joan", hash); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after a password reset: err = %v, want ErrNoSession", err)
	}

	token, _, err = s.LogIn(ctx, "joan", "a brand new password")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteUser(ctx, "joan"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Authenticate(ctx, token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after removing the user: err = %v, want ErrNoSession", err)
	}
}
