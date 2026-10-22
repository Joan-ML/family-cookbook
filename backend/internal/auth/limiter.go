package auth

import (
	"sync"
	"time"
)

// limiter slows down password guessing: after maxFailures failed logins for a
// username within window, further attempts are refused until the window ends.
// It's in memory, so a restart resets it, which is fine for a small server.
type limiter struct {
	mu       sync.Mutex
	failures map[string]*failures
}

type failures struct {
	count int
	since time.Time
}

const (
	maxFailures   = 5
	failureWindow = 15 * time.Minute
)

// retryAfter returns how long username must wait before trying again, or 0.
func (l *limiter) retryAfter(username string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.failures[username]
	if f == nil || f.count < maxFailures {
		return 0
	}
	return max(f.since.Add(failureWindow).Sub(now), 0)
}

func (l *limiter) fail(username string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.failures == nil {
		l.failures = map[string]*failures{}
	}
	// Forget windows that are over, so the map can't grow without bound.
	for name, f := range l.failures {
		if now.Sub(f.since) >= failureWindow {
			delete(l.failures, name)
		}
	}
	f := l.failures[username]
	if f == nil {
		f = &failures{since: now}
		l.failures[username] = f
	}
	f.count++
}

func (l *limiter) succeed(username string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, username)
}
