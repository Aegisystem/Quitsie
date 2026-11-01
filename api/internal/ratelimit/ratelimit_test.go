package ratelimit_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/ratelimit"
)

// clock is a time source the test moves.
type clock struct{ now time.Time }

func newClock() *clock { return &clock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)} }

func (c *clock) Now() time.Time          { return c.now }
func (c *clock) Advance(d time.Duration) { c.now = c.now.Add(d) }

// spend uses n events of key and fails if any is refused.
func spend(t *testing.T, l *ratelimit.Limiter, key string, n int) {
	t.Helper()
	for i := range n {
		if ok, _ := l.Allow(key); !ok {
			t.Fatalf("event %d of %s was refused", i+1, key)
		}
	}
}

func TestAKeyGetsAMinuteOfEventsAtOnceAndThenOneAtATime(t *testing.T) {
	c := newClock()
	l := ratelimit.New(60, time.Minute, 100, c.Now)
	spend(t, l, "a", 60)

	ok, retryAfter := l.Allow("a")

	if ok || retryAfter != time.Second {
		t.Errorf("event 61: ok = %v, retryAfter = %v, want refused for 1s", ok, retryAfter)
	}

	c.Advance(time.Second)

	spend(t, l, "a", 1)
	if ok, _ := l.Allow("a"); ok {
		t.Error("two events in the second that gives one back, want the second refused")
	}
}

func TestAnEventCanCostSeveral(t *testing.T) {
	c := newClock()
	l := ratelimit.New(60, time.Hour, 100, c.Now)
	if ok, _ := l.AllowN("a", 59); !ok {
		t.Fatal("59 of 60 were refused")
	}

	ok, retryAfter := l.AllowN("a", 2)

	if ok || retryAfter != time.Minute {
		t.Errorf("2 more: ok = %v, retryAfter = %v, want refused for a minute, the time one takes to come back", ok, retryAfter)
	}

	c.Advance(time.Minute)

	if ok, _ := l.AllowN("a", 2); !ok {
		t.Error("2 more after a minute were refused")
	}
}

func TestARefusedEventCostsNothing(t *testing.T) {
	c := newClock()
	l := ratelimit.New(60, time.Minute, 100, c.Now)
	spend(t, l, "a", 60)
	for range 1000 {
		l.Allow("a")
	}

	c.Advance(time.Second)

	spend(t, l, "a", 1)
}

func TestKeysCountApart(t *testing.T) {
	c := newClock()
	l := ratelimit.New(60, time.Minute, 100, c.Now)
	spend(t, l, "a", 60)

	spend(t, l, "b", 60)
}

func TestAKeyThatStoppedIsForgotten(t *testing.T) {
	c := newClock()
	l := ratelimit.New(60, time.Minute, 100, c.Now)
	for i := range 50 {
		spend(t, l, fmt.Sprintf("key-%d", i), 60)
	}

	c.Advance(2 * time.Minute)
	spend(t, l, "another", 1)

	if got := l.Keys(); got != 1 {
		t.Errorf("Keys() = %d, want 1: the 50 old ones have their allowance back", got)
	}
}

func TestAKeyThatStillWaitsIsNotForgotten(t *testing.T) {
	c := newClock()
	l := ratelimit.New(60, time.Minute, 100, c.Now)
	spend(t, l, "a", 1)
	c.Advance(59 * time.Second)
	spend(t, l, "a", 60)
	c.Advance(time.Second)

	spend(t, l, "b", 1) // the time to clean up comes before a's allowance is back
	spend(t, l, "a", 1)

	if ok, _ := l.Allow("a"); ok {
		t.Error("a got more than its allowance back")
	}
}

func TestItNeverRemembersMoreKeysThanItsMaximum(t *testing.T) {
	c := newClock()
	l := ratelimit.New(60, time.Minute, 100, c.Now)

	for i := range 10_000 {
		spend(t, l, fmt.Sprintf("key-%d", i), 1)
		if got := l.Keys(); got > 100 {
			t.Fatalf("after %d keys, Keys() = %d, want at most 100", i+1, got)
		}
	}
}

func TestItCanBeUsedFromManyRequestsAtOnce(t *testing.T) {
	l := ratelimit.New(60, time.Minute, 10, time.Now)
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range 500 {
				l.Allow(fmt.Sprintf("key-%d", (g*i)%50))
			}
		})
	}
	wg.Wait()

	if got := l.Keys(); got > 10 {
		t.Errorf("Keys() = %d, want at most 10", got)
	}
}
