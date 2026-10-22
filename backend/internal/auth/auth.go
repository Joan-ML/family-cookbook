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
	// SetupLinkTTL is how long a setup link works, if unused.
	SetupLinkTTL = 48 * time.Hour
	// renewAfter limits how often a session in use is extended, so requests
	// don't each write to the database.
	renewAfter = 24 * time.Hour
)

var (
	ErrInvalidCredentials = errors.New("wrong username or password")
	ErrNoSession          = errors.New("no valid session")
	ErrNoSuchUser         = errors.New("no such user")
	ErrInvalidLink        = errors.New("this link is invalid, already used or expired; ask for a new one")
)

// BadPasswordError is returned when a new password breaks the rules.
type BadPasswordError struct{ Reason string }

func (e *BadPasswordError) Error() string { return e.Reason }

// Store is the storage the auth service needs.
type Store interface {
	store.Users
	store.Sessions
	store.SetupTokens
}

// TooManyAttemptsError is returned by LogIn while a username is locked out.
type TooManyAttemptsError struct{ RetryAfter time.Duration }

func (e *TooManyAttemptsError) Error() string {
	return fmt.Sprintf("too many failed attempts; try again in %s", e.RetryAfter.Round(time.Second))
}

// Service logs users in and out, checks sessions and manages passwords.
type Service struct {
	store   Store
	now     func() time.Time
	limiter limiter

	dummyOnce sync.Once
	dummyHash string
}

func NewService(st Store, now func() time.Time) *Service {
	return &Service{store: st, now: now}
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

	user, err := s.store.GetUser(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		// Hash anyway, so the response time doesn't reveal which usernames exist.
		_, _ = VerifyPassword(s.dummy(), password)
		s.limiter.fail(name, now)
		return "", store.Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return "", store.Session{}, err
	}
	if user.PasswordHash == "" {
		// No password yet (or reset): only a setup link gets in.
		_, _ = VerifyPassword(s.dummy(), password)
		s.limiter.fail(name, now)
		return "", store.Session{}, ErrInvalidCredentials
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

	return s.startSession(ctx, user, now)
}

// startSession creates a session for user and returns its token.
func (s *Service) startSession(ctx context.Context, user store.User, now time.Time) (string, store.Session, error) {
	// Logins are rare, so they're a good moment to clear out old sessions.
	if err := s.store.DeleteExpiredSessions(ctx, now); err != nil {
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
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return "", store.Session{}, err
	}
	return token, sess, nil
}

// IssueSetupLink creates a one-time token with which the user chooses a
// password, replacing any earlier one. It returns the token, which only goes
// into the link, and when it expires.
func (s *Service) IssueSetupLink(ctx context.Context, username string) (string, time.Time, error) {
	name, err := NormalizeUsername(username)
	if err != nil {
		return "", time.Time{}, ErrNoSuchUser
	}
	user, err := s.store.GetUser(ctx, name)
	if errors.Is(err, store.ErrNotFound) {
		return "", time.Time{}, ErrNoSuchUser
	}
	if err != nil {
		return "", time.Time{}, err
	}
	token, err := newToken()
	if err != nil {
		return "", time.Time{}, err
	}
	now := s.now()
	expires := now.Add(SetupLinkTTL)
	if err := s.store.CreateSetupToken(ctx, hashToken(token), user.ID, now, expires); err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

// ResetPassword disables the user's password, logs them out everywhere and
// issues a setup link for a new one.
func (s *Service) ResetPassword(ctx context.Context, username string) (string, time.Time, error) {
	name, err := NormalizeUsername(username)
	if err != nil {
		return "", time.Time{}, ErrNoSuchUser
	}
	if err := s.store.SetPassword(ctx, name, ""); errors.Is(err, store.ErrNotFound) {
		return "", time.Time{}, ErrNoSuchUser
	} else if err != nil {
		return "", time.Time{}, err
	}
	return s.IssueSetupLink(ctx, name)
}

// CompleteSetup sets a password with a setup link's token and logs the user
// in. The token stops working, and the user's other sessions end.
func (s *Service) CompleteSetup(ctx context.Context, token, password string) (string, store.Session, error) {
	if err := CheckNewPassword(password); err != nil {
		return "", store.Session{}, &BadPasswordError{Reason: err.Error()}
	}
	hash, err := HashPassword(password)
	if err != nil {
		return "", store.Session{}, err
	}
	now := s.now()
	user, err := s.store.UseSetupToken(ctx, hashToken(token), hash, now)
	if errors.Is(err, store.ErrNotFound) {
		return "", store.Session{}, ErrInvalidLink
	}
	if err != nil {
		return "", store.Session{}, err
	}
	s.limiter.succeed(user.Username)
	return s.startSession(ctx, user, now)
}

// ChangePassword changes the password of the session's user, who must give
// their current one. Their other sessions end; this one stays.
func (s *Service) ChangePassword(ctx context.Context, sess store.Session, current, next string) error {
	now := s.now()
	if wait := s.limiter.retryAfter(sess.Username, now); wait > 0 {
		return &TooManyAttemptsError{RetryAfter: wait}
	}
	user, err := s.store.GetUser(ctx, sess.Username)
	if err != nil {
		return err
	}
	ok, err := VerifyPassword(user.PasswordHash, current)
	if err != nil {
		return err
	}
	if !ok {
		s.limiter.fail(sess.Username, now)
		return ErrInvalidCredentials
	}
	if err := CheckNewPassword(next); err != nil {
		return &BadPasswordError{Reason: err.Error()}
	}
	hash, err := HashPassword(next)
	if err != nil {
		return err
	}
	return s.store.ChangePassword(ctx, user.ID, hash, sess.TokenHash)
}

// Authenticate returns the session for token. renewed reports whether its
// expiry was pushed back, in which case the cookie should be sent again.
func (s *Service) Authenticate(ctx context.Context, token string) (sess store.Session, renewed bool, err error) {
	if token == "" {
		return store.Session{}, false, ErrNoSession
	}
	sess, err = s.store.GetSession(ctx, hashToken(token))
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
		if err := s.store.ExtendSession(ctx, sess.TokenHash, sess.ExpiresAt); err != nil {
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
	return s.store.DeleteSession(ctx, hashToken(token))
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
