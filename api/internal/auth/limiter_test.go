package auth

import (
	"fmt"
	"testing"
	"time"
)

var someTime = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// Someone over the limit of their IP can send a new email each time. Each one
// used to stay in memory for up to two hours, although nothing was sent to it.
func TestARefusedRequestLeavesNoKeyBehind(t *testing.T) {
	l := newLimiter(time.Hour)
	ip := quota{"ip:203.0.113.7", 2}
	for i := range 2 {
		if ok, _ := l.allow(someTime, quota{fmt.Sprintf("email:%d", i), 5}, ip); !ok {
			t.Fatalf("request %d was refused", i+1)
		}
	}

	for i := range 1000 {
		if ok, _ := l.allow(someTime, quota{fmt.Sprintf("email:over-%d", i), 5}, ip); ok {
			t.Fatalf("request %d over the limit was allowed", i+1)
		}
	}

	if got := len(l.hits); got != 3 {
		t.Errorf("keys = %d, want the 2 emails and the IP", got)
	}
}

func TestTheLimiterNeverRemembersMoreKeysThanItsMaximum(t *testing.T) {
	l := newLimiter(time.Hour)
	l.maxKeys = 100

	for i := range 10_000 {
		l.allow(someTime, quota{fmt.Sprintf("email:%d", i), 5}, quota{fmt.Sprintf("ip:%d", i), 30})
		if got := len(l.hits); got > 100 {
			t.Fatalf("after %d requests, keys = %d, want at most 100", i+1, got)
		}
	}
}

func TestAKeyStillCountsWhileThereIsRoom(t *testing.T) {
	l := newLimiter(time.Hour)
	key := quota{"email:ana", 2}
	for range 2 {
		l.allow(someTime, key)
	}

	ok, retryAfter := l.allow(someTime.Add(time.Minute), key)

	if ok || retryAfter != 59*time.Minute {
		t.Errorf("ok = %v, retryAfter = %v, want refused for 59 minutes", ok, retryAfter)
	}
}
