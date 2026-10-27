package auth

import (
	"sync"
	"time"
)

// Limits on sending login codes, so nobody can flood an inbox or use
// Quitsie to send spam. The IP limit is looser because many people can
// share one address behind a carrier's NAT.
const (
	sendWindow      = time.Hour
	maxSendsPerMail = 5
	maxSendsPerIP   = 30
)

// limiter counts events per key over a sliding window. It lives in memory,
// so a restart forgets the counts; that is acceptable for a single binary.
type limiter struct {
	mu        sync.Mutex
	window    time.Duration
	hits      map[string][]time.Time
	lastSweep time.Time
}

func newLimiter(window time.Duration) *limiter {
	return &limiter{window: window, hits: make(map[string][]time.Time)}
}

type quota struct {
	key string
	max int
}

// allow records an event under every quota's key, or under none of them if
// any key already reached its maximum within the window. Then it reports
// how long until every such key has room again.
func (l *limiter) allow(now time.Time, quotas ...quota) (ok bool, retryAfter time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.sweep(now)

	since := now.Add(-l.window)
	for _, q := range quotas {
		hits := recent(l.hits[q.key], since)
		l.hits[q.key] = hits
		if len(hits) >= q.max {
			// The key has room once the event that filled it leaves the window.
			retryAfter = max(retryAfter, hits[len(hits)-q.max].Add(l.window).Sub(now))
		}
	}
	if retryAfter > 0 {
		return false, retryAfter
	}
	for _, q := range quotas {
		l.hits[q.key] = append(l.hits[q.key], now)
	}
	return true, 0
}

// sweep drops keys with no recent events, at most once per window, so the
// map does not grow with every email and IP ever seen.
func (l *limiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	since := now.Add(-l.window)
	for key, times := range l.hits {
		if times = recent(times, since); len(times) == 0 {
			delete(l.hits, key)
		} else {
			l.hits[key] = times
		}
	}
	l.lastSweep = now
}

// recent drops the times at or before since; times is in ascending order.
func recent(times []time.Time, since time.Time) []time.Time {
	i := 0
	for i < len(times) && !times[i].After(since) {
		i++
	}
	return times[i:]
}
