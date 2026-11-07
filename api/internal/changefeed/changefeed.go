// Package changefeed tells the server, as it happens, that a ledger changed.
// The database sends a notification for every write (migration 00002), and a
// Feed passes it on to whoever subscribed: the open devices (SSE) and the push
// notifications (ADR 0013).
//
// A Change is a hint to read the database again, not the change itself.
// Notifications sent while the Feed is not connected are lost, so after each
// connection it sends a Change for every ledger, and a subscriber that falls
// behind loses Changes rather than holding up the others.
package changefeed

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
)

// channel is the one the trigger of migration 00002 notifies.
const channel = "quitsie_changes"

// Change says that the ledger LedgerID changed at the sync version Version.
type Change struct {
	LedgerID string `json:"ledgerId"`
	Version  int64  `json:"version"`
}

// AnyLedger reports whether the Change stands for every ledger: the Feed has
// just connected, and anything may have changed in the meantime.
func (c Change) AnyLedger() bool { return c.LedgerID == "" }

// buffered is how many Changes a subscriber can fall behind before it loses one.
const buffered = 256

// Feed listens to the database and passes each Change to its subscribers.
type Feed struct {
	url    string
	logger *slog.Logger

	mu          sync.Mutex
	subscribers map[chan Change]struct{}
}

// New returns a Feed on the database at url. It listens once Run starts.
func New(url string, logger *slog.Logger) *Feed {
	return &Feed{url: url, logger: logger, subscribers: map[chan Change]struct{}{}}
}

// Subscribe returns the Changes from now on, until cancel is called.
func (f *Feed) Subscribe() (changes <-chan Change, cancel func()) {
	ch := make(chan Change, buffered)
	f.mu.Lock()
	f.subscribers[ch] = struct{}{}
	f.mu.Unlock()
	var once sync.Once
	return ch, func() {
		once.Do(func() {
			f.mu.Lock()
			delete(f.subscribers, ch)
			f.mu.Unlock()
		})
	}
}

func (f *Feed) publish(c Change) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for ch := range f.subscribers {
		select {
		case ch <- c:
		default:
			// A slow subscriber loses this one; its next read of the database
			// catches up anyway.
		}
	}
}

// Run listens until ctx ends, reconnecting with a growing wait when the
// connection drops.
func (f *Feed) Run(ctx context.Context) {
	wait := time.Second
	for {
		connected, err := f.listen(ctx)
		if ctx.Err() != nil {
			return
		}
		if connected {
			wait = time.Second
		}
		f.logger.Warn("change feed disconnected, reconnecting", "err", err, "in", wait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, time.Minute)
	}
}

// listen holds one connection until it fails, and says whether it got to listen.
func (f *Feed) listen(ctx context.Context) (connected bool, err error) {
	conn, err := pgx.Connect(ctx, f.url)
	if err != nil {
		return false, err
	}
	defer conn.Close(context.Background())
	if _, err := conn.Exec(ctx, "LISTEN "+channel); err != nil {
		return false, err
	}
	// What changed while it was not listening is unknown.
	f.publish(Change{})
	for {
		n, err := conn.WaitForNotification(ctx)
		if err != nil {
			return true, err
		}
		var c Change
		if err := json.Unmarshal([]byte(n.Payload), &c); err != nil || c.LedgerID == "" {
			f.logger.Error("change feed: unreadable notification", "payload", n.Payload)
			continue
		}
		f.publish(c)
	}
}
