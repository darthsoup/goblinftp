package auth

import (
	"sync"
	"time"
)

type throttleEntry struct {
	attempts  int
	expiresAt time.Time
}

// Throttle tracks per-key failed login attempts and enforces cooldown periods.
type Throttle struct {
	mu      sync.Mutex
	entries map[string]*throttleEntry
}

// NewThrottle creates a new Throttle.
func NewThrottle() *Throttle {
	return &Throttle{entries: make(map[string]*throttleEntry)}
}

// Record increments the attempt counter for key and sets or extends the cooldown.
func (t *Throttle) Record(key string, cooldown time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()

	e := t.entries[key]
	if e == nil {
		e = &throttleEntry{}
		t.entries[key] = e
	}

	e.attempts++
	e.expiresAt = time.Now().Add(cooldown)
}

// IsThrottled reports whether key has reached maxAttempts within its cooldown window.
func (t *Throttle) IsThrottled(key string, maxAttempts int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	e := t.entries[key]
	if e == nil {
		return false
	}
	if time.Now().After(e.expiresAt) {
		delete(t.entries, key)
		return false
	}

	return e.attempts >= maxAttempts
}

// Reset removes all throttle state for key.
func (t *Throttle) Reset(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.entries, key)
}
