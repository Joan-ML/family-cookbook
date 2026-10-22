// Package auth handles accounts, passwords and login sessions, independent of
// HTTP: the API layer turns tokens into cookies.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"family-cookbook/backend/internal/store"
)

const (
	// SessionTTL is how long a session lasts without being used.
	SessionTTL = 30 * 24 * time.Hour
	// renewAfter limits how often a session in use is extended, so requests
	// don't each write to the database.
	renewAfter = 24 * time.Hour
)

var (
	ErrInvalidCredentials = errors.New("wrong username or password")
	ErrNoSession          = errors.New("no valid session")
)

// TooManyAttemptsError is returned by LogIn while a username is locked out.
type TooManyAttemptsError struct{ RetryAfter time.Duration }

func (e *TooManyAttemptsError) Error() string {
	return fmt.Sprintf("too many failed attempts; try again in %s", e.RetryAfter.Round(time.Second))
}

// Service logs users in and out and checks sessions.
type Service struct {
	users    store.Users
	sessions store.Sessions
	now      func() time.Time
	limiter  limiter

	dummyOnce sync.Once
	dummyHash string
}

func NewService(users store.Users, sessions store.Sessions, now func() time.Time) *Service {
	return &Service{users: users, sessions: sessions, now: now}
}

// LogIn checks a username and password and starts a session. It returns the
// session token, which only the client keeps.
func (s *Service) LogIn(ctx context.Context, username, password string) (string, store.Session, error) {
	now := s.now()
	name, err := NormalizeUsername(username)
	if err != nil {
		return "", store.Session{}, ErrInvalidCredentials
	}
	if wait := s.limiter.retryAfter(name, now); wait > 0 {
		return "", store.Session{}, &TooManyAttemptsError{RetryAfter: wait}
	}

	user, err := s.users.GetUser(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		// Hash anyway, so the response time doesn't reveal which usernames exist.
		_, _ = VerifyPassword(s.dummy(), password)
		s.limiter.fail(name, now)
		return "", store.Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", store.Session{}, err
	}
	ok, err := VerifyPassword(user.PasswordHash, password)
	if err != nil {
		return "", store.Session{}, err
	}
	if !ok {
		s.limiter.fail(name, now)
		return "", store.Session{}, ErrInvalidCredentials
	}
	s.limiter.succeed(name)

	// Logins are rare, so they're a good moment to clear out old sessions.
	if err := s.sessions.DeleteExpiredSessions(ctx, now); err != nil {
		return "", store.Session{}, err
	}
	token, err := newToken()
	if err != nil {
		return "", store.Session{}, err
	}
	sess := store.Session{
		TokenHash: hashToken(token),
		UserID:    user.ID,
		Username:  user.Username,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionTTL),
	}
	if err := s.sessions.CreateSession(ctx, sess); err != nil {
		return "", store.Session{}, err
	}
	return token, sess, nil
}

// Authenticate returns the session for token. renewed reports whether its
// expiry was pushed back, in which case the cookie should be sent again.
func (s *Service) Authenticate(ctx context.Context, token string) (sess store.Session, renewed bool, err error) {
	if token == "" {
		return store.Session{}, false, ErrNoSession
	}
	sess, err = s.sessions.GetSession(ctx, hashToken(token))
	if errors.Is(err, store.ErrNotFound) {
		return store.Session{}, false, ErrNoSession
	}
	if err != nil {
		return store.Session{}, false, err
	}
	now := s.now()
	if !now.Before(sess.ExpiresAt) {
		return store.Session{}, false, ErrNoSession
	}
	if sess.ExpiresAt.Sub(now) < SessionTTL-renewAfter {
		sess.ExpiresAt = now.Add(SessionTTL)
		if err := s.sessions.ExtendSession(ctx, sess.TokenHash, sess.ExpiresAt); err != nil {
			return store.Session{}, false, err
		}
		renewed = true
	}
	return sess, renewed, nil
}

// LogOut ends the session for token, if there is one.
func (s *Service) LogOut(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.sessions.DeleteSession(ctx, hashToken(token))
}

// dummy returns a hash to verify against for unknown users.
func (s *Service) dummy() string {
	s.dummyOnce.Do(func() {
		s.dummyHash, _ = HashPassword("not a real password")
	})
	return s.dummyHash
}

func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
