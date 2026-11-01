package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// request sends one request through h and returns its status.
func request(ctx context.Context, h http.Handler) int {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx))
	return rec.Code
}

// held is a gate of one whose only place stays taken until the test releases it.
func held(t *testing.T, wait time.Duration) (h http.Handler, release func()) {
	t.Helper()
	inside, letGo := make(chan struct{}), make(chan struct{})
	first := true
	var once sync.Once
	h = gate(1, wait, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		if first {
			first = false
			close(inside)
			<-letGo
		}
	}))
	done := make(chan struct{})
	go func() {
		defer close(done)
		request(context.Background(), h)
	}()
	<-inside
	release = func() {
		once.Do(func() {
			close(letGo)
			<-done
		})
	}
	t.Cleanup(release)
	return h, release
}

func TestAFullGateAnswers429AfterWaiting(t *testing.T) {
	h, _ := held(t, 10*time.Millisecond)

	if got := request(context.Background(), h); got != http.StatusTooManyRequests {
		t.Errorf("code = %d, want 429", got)
	}
}

func TestAGateLetsTheNextOneInWhenThereIsRoom(t *testing.T) {
	h, release := held(t, time.Minute)
	release()

	if got := request(context.Background(), h); got != http.StatusOK {
		t.Errorf("code = %d, want 200", got)
	}
}

func TestSomeoneWhoGivesUpLeavesTheGate(t *testing.T) {
	h, _ := held(t, time.Minute)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	finished := make(chan struct{})
	go func() {
		defer close(finished)
		request(ctx, h)
	}()

	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("it is still waiting")
	}
}

func TestAKeyGetsOneRequestAtATime(t *testing.T) {
	inside, letGo := make(chan struct{}), make(chan struct{})
	h := onePerKey(func(r *http.Request) (string, bool) {
		key := r.URL.Query().Get("key")
		return key, key != ""
	}, http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Has("hold") {
			close(inside)
			<-letGo
		}
	}))
	send := func(target string) int {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec.Code
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		send("/?key=a&hold")
	}()
	<-inside

	if got := send("/?key=a"); got != http.StatusTooManyRequests {
		t.Errorf("same key: code = %d, want 429", got)
	}
	if got := send("/?key=b"); got != http.StatusOK {
		t.Errorf("another key: code = %d, want 200", got)
	}
	if got := send("/"); got != http.StatusOK {
		t.Errorf("no key: code = %d, want 200", got)
	}

	close(letGo)
	<-done
	if got := send("/?key=a"); got != http.StatusOK {
		t.Errorf("same key afterwards: code = %d, want 200", got)
	}
}
