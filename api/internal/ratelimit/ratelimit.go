// Package ratelimit counts events per key in memory, for the limits of requests
// per IP and of bytes per User. A restart forgets the counts, which is
// acceptable for a single binary.
package ratelimit

import (
	"sync"
	"time"
)

// Limiter gives each key a bucket of events that refills evenly over a window,
// so a key can spend a whole window's worth at once and then one at a time.
//
// Its memory has a ceiling: each key costs one timestamp, a key whose bucket
// is full again is forgotten, and there are never more than maxKeys.
type Limiter struct {
	mu       sync.Mutex
	window   time.Duration
	interval time.Duration
	maxKeys  int
	now      func() time.Time
	// fullAt is when the bucket of each key will be full again, which says
	// everything about it: the bucket lacks one event per interval between
	// now and then. A time in the past is the same as a key never seen.
	fullAt    map[string]time.Time
	nextSweep time.Time
}

// New returns a Limiter of events per key per window that remembers at most
// maxKeys keys and reads the time from now.
func New(events int, window time.Duration, maxKeys int, now func() time.Time) *Limiter {
	return &Limiter{
		window:   window,
		interval: window / time.Duration(events),
		maxKeys:  maxKeys,
		now:      now,
		fullAt:   make(map[string]time.Time),
	}
}

// Allow spends one event of key. If its bucket is empty it spends nothing and
// reports how long until there is an event to spend.
func (l *Limiter) Allow(key string) (ok bool, retryAfter time.Duration) {
	return l.AllowN(key, 1)
}

// AllowN spends n events of key at once, or none of them, like Allow.
func (l *Limiter) AllowN(key string, n int) (ok bool, retryAfter time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)

	full, known := l.fullAt[key]
	if !known || full.Before(now) {
		full = now
	}
	full = full.Add(time.Duration(n) * l.interval)
	if wait := full.Sub(now) - l.window; wait > 0 {
		return false, wait
	}
	if !known {
		l.makeRoom()
	}
	l.fullAt[key] = full
	return true, 0
}

// Keys is how many keys it remembers now.
func (l *Limiter) Keys() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.fullAt)
}

// sweep forgets the keys with a full bucket, at most once per window, so it
// costs nothing on most events.
func (l *Limiter) sweep(now time.Time) {
	if now.Before(l.nextSweep) {
		return
	}
	for key, full := range l.fullAt {
		if !full.After(now) {
			delete(l.fullAt, key)
		}
	}
	l.nextSweep = now.Add(l.window)
}

// makeRoom forgets one key if there are maxKeys already. That takes that many
// addresses within a minute, so it means an attack from many places. Whichever
// key Go's map order picks starts over with a full bucket: under that attack
// the limit gets looser for a few, the memory stays where it was, and nobody
// is refused who did not earn it.
func (l *Limiter) makeRoom() {
	if len(l.fullAt) < l.maxKeys {
		return
	}
	for key := range l.fullAt {
		delete(l.fullAt, key)
		return
	}
}
