package auth

import (
	"errors"
	"regexp"
	"strings"
	"unicode/utf8"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9._-]{1,64}$`)

// NormalizeUsername lowercases a username and checks it: 1–64 characters out
// of lowercase letters, digits, ".", "_" and "-". Usernames are compared in
// this form, so "Joan" and "joan" are the same user.
func NormalizeUsername(s string) (string, error) {
	u := strings.ToLower(strings.TrimSpace(s))
	if !usernamePattern.MatchString(u) {
		return "", errors.New(`usernames are 1–64 characters: letters, digits, ".", "_" and "-"`)
	}
	return u, nil
}

// MinPasswordLength is enforced when setting passwords, not when logging in.
const MinPasswordLength = 10

// CheckNewPassword checks a password being set.
func CheckNewPassword(p string) error {
	switch n := utf8.RuneCountInString(p); {
	case n < MinPasswordLength:
		return errors.New("passwords need at least 10 characters")
	case len(p) > 1024:
		return errors.New("passwords can be at most 1024 bytes")
	}
	return nil
}
