package auth

import (
	"sync"
	"time"
)

// Limits on sending login codes, so nobody can flood an inbox or use
// Quitsie to send spam. The IP limit is looser because many people can
// share one address behind a carrier's NAT, but each code is an email out of
// the server's budget, so one address gets a few people's worth.
//
// Anybody can spend the limit per email, so a known device has its own
// quota instead (ADR 0011). Other browsers also have a smaller one per pair
// of IP and email, so that one sender cannot take all of an email's codes
// from someone at another address.
const (
	sendWindow             = time.Hour
	maxSendsPerMail        = 5
	maxSendsPerPair        = 3
	maxSendsPerIP          = 10
	maxSendsPerKnownDevice = 5
	// maxVerificationsPerIP stops one sender from spreading guesses over many
	// emails, whose codes it may have asked for from other addresses.
	maxVerificationsPerIP = 60
	// maxLimiterKeys bounds the memory of a limiter to some 80 MB. Reaching it
	// takes tens of thousands of addresses within an hour.
	maxLimiterKeys = 100_000
)

// limiter counts events per key over a sliding window. It lives in memory,
// so a restart forgets the counts; that is acceptable for a single binary.
type limiter struct {
	mu        sync.Mutex
	window    time.Duration
	maxKeys   int
	hits      map[string][]time.Time
	lastSweep time.Time
}

func newLimiter(window time.Duration) *limiter {
	return &limiter{window: window, maxKeys: maxLimiterKeys, hits: make(map[string][]time.Time)}
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
	if retryAfter := l.untilRoom(now, quotas); retryAfter > 0 {
		return false, retryAfter
	}
	since := now.Add(-l.window)
	for _, q := range quotas {
		hits, known := l.hits[q.key]
		if !known {
			l.makeRoom()
		}
		l.hits[q.key] = append(recent(hits, since), now)
	}
	return true, 0
}

// wait reports how long until every quota's key has room for one more event,
// zero if they have room now, as allow would. It records nothing, so a caller
// can refuse early and still call allow once nothing else can refuse.
func (l *limiter) wait(now time.Time, quotas ...quota) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.untilRoom(now, quotas)
}

func (l *limiter) untilRoom(now time.Time, quotas []quota) (retryAfter time.Duration) {
	l.sweep(now)
	since := now.Add(-l.window)
	for _, q := range quotas {
		if hits := recent(l.hits[q.key], since); len(hits) >= q.max {
			// The key has room once the event that filled it leaves the window.
			retryAfter = max(retryAfter, hits[len(hits)-q.max].Add(l.window).Sub(now))
		}
	}
	return retryAfter
}

// makeRoom forgets one key if there are maxKeys already, which means an attack
// from many places. Whichever key Go's map order picks starts over: the limit
// gets looser for a few, the memory stays where it was, and nobody is refused
// who did not earn it.
func (l *limiter) makeRoom() {
	if len(l.hits) < l.maxKeys {
		return
	}
	for key := range l.hits {
		delete(l.hits, key)
		return
	}
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
