package api

import (
	"strings"
	"sync"
	"time"
)

const (
	loginUserLimit = 5
	loginIPLimit   = 20
	loginWindow    = 15 * time.Minute
)

type loginThrottleEntry struct {
	count   int
	resetAt time.Time
}

// LoginThrottle counts failed logins per username and per IP address. It is
// in-memory and therefore per-process.
type LoginThrottle struct {
	mutex       sync.Mutex
	now         func() time.Time
	entries     map[string]loginThrottleEntry
	nextSweepAt time.Time
}

func NewLoginThrottle() *LoginThrottle {
	throttle := &LoginThrottle{
		now:     time.Now,
		entries: make(map[string]loginThrottleEntry),
	}
	throttle.nextSweepAt = throttle.now().Add(loginWindow)
	return throttle
}

// Check reports whether ip or username is currently blocked and how long the
// caller should wait.
func (throttle *LoginThrottle) Check(ip, username string) (time.Duration, bool) {
	now := throttle.now()

	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	throttle.sweepExpired(now)

	var retryAfter time.Duration
	blocked := false

	if entry, exists := throttle.entries[loginUsernameKey(username)]; exists &&
		now.Before(entry.resetAt) &&
		entry.count >= loginUserLimit {
		blocked = true
		retryAfter = entry.resetAt.Sub(now)
	}
	if entry, exists := throttle.entries[loginIPKey(ip)]; exists &&
		now.Before(entry.resetAt) &&
		entry.count >= loginIPLimit {
		blocked = true
		if remaining := entry.resetAt.Sub(now); remaining > retryAfter {
			retryAfter = remaining
		}
	}

	if !blocked {
		return 0, false
	}
	return retryAfter, true
}

// RecordFailure increments both counters.
func (throttle *LoginThrottle) RecordFailure(ip, username string) {
	now := throttle.now()

	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	throttle.sweepExpired(now)

	if key := loginUsernameKey(username); key != "user:" {
		throttle.increment(key, now)
	}
	throttle.increment(loginIPKey(ip), now)
}

// RecordSuccess clears the username counter but keeps the IP counter so a
// known valid password cannot reset an attacker's IP limit.
func (throttle *LoginThrottle) RecordSuccess(username string) {
	key := loginUsernameKey(username)
	if key == "user:" {
		return
	}

	throttle.mutex.Lock()
	defer throttle.mutex.Unlock()
	delete(throttle.entries, key)
}

func (throttle *LoginThrottle) increment(key string, now time.Time) {
	entry, exists := throttle.entries[key]
	if !exists || !now.Before(entry.resetAt) {
		entry = loginThrottleEntry{resetAt: now.Add(loginWindow)}
	}
	entry.count++
	throttle.entries[key] = entry
}

func (throttle *LoginThrottle) sweepExpired(now time.Time) {
	if now.Before(throttle.nextSweepAt) {
		return
	}
	for key, entry := range throttle.entries {
		if !now.Before(entry.resetAt) {
			delete(throttle.entries, key)
		}
	}
	throttle.nextSweepAt = now.Add(loginWindow)
}

func loginUsernameKey(username string) string {
	return "user:" + strings.ToLower(strings.TrimSpace(username))
}

func loginIPKey(ip string) string {
	return "ip:" + ip
}
