package changefeed_test

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/changefeed"
	"github.com/Aegisystem/Quitsie/api/internal/storetest"
)

// listening starts a Feed on db and waits until it listens.
func listening(t *testing.T, db *sql.DB) (*changefeed.Feed, <-chan changefeed.Change) {
	t.Helper()
	feed := changefeed.New(storetest.URLOf(t, db), slog.New(slog.NewTextHandler(io.Discard, nil)))
	changes, cancel := feed.Subscribe()
	t.Cleanup(cancel)
	ctx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	go feed.Run(ctx)
	if c := next(t, changes); !c.AnyLedger() {
		t.Fatalf("first change = %+v, want the one for every ledger", c)
	}
	return feed, changes
}

func next(t *testing.T, changes <-chan changefeed.Change) changefeed.Change {
	t.Helper()
	select {
	case c := <-changes:
		return c
	case <-time.After(5 * time.Second):
		t.Fatal("no change arrived")
		return changefeed.Change{}
	}
}

func nothingMore(t *testing.T, changes <-chan changefeed.Change) {
	t.Helper()
	select {
	case c := <-changes:
		t.Fatalf("unexpected change %+v", c)
	case <-time.After(200 * time.Millisecond):
	}
}

func insertGroup(t *testing.T, db *sql.DB, id string, version int64) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO ledgers (id, kind, name, currency, version, created_at) VALUES ($1, 'group', 'Casa', 'COP', $2, now())`, id, version)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAWriteToALedgerArrivesWithItsVersion(t *testing.T) {
	db := storetest.Open(t)
	_, changes := listening(t, db)

	insertGroup(t, db, "ledger-1", 7)

	if c := next(t, changes); c != (changefeed.Change{LedgerID: "ledger-1", Version: 7}) {
		t.Errorf("change = %+v", c)
	}
}

func TestATransactionWithManyRowsOfALedgerSendsOneChange(t *testing.T) {
	db := storetest.Open(t)
	_, changes := listening(t, db)

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO ledgers (id, kind, name, currency, version, created_at) VALUES ('ledger-2', 'group', 'Viaje', 'USD', 9, now())`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"member-a", "member-b"} {
		if _, err := tx.Exec(`INSERT INTO members (id, ledger_id, name, version, created_at) VALUES ($1, 'ledger-2', 'Ana', 9, now())`, id); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if c := next(t, changes); c != (changefeed.Change{LedgerID: "ledger-2", Version: 9}) {
		t.Errorf("change = %+v", c)
	}
	nothingMore(t, changes)
}

func TestARolledBackWriteSendsNothing(t *testing.T) {
	db := storetest.Open(t)
	_, changes := listening(t, db)

	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(`INSERT INTO ledgers (id, kind, name, currency, version, created_at) VALUES ('ledger-3', 'group', 'Viaje', 'USD', 3, now())`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	nothingMore(t, changes)
}

func TestACancelledSubscriberGetsNothingMore(t *testing.T) {
	db := storetest.Open(t)
	feed, changes := listening(t, db)
	other, cancel := feed.Subscribe()
	cancel()
	cancel()

	insertGroup(t, db, "ledger-4", 4)

	next(t, changes)
	select {
	case c := <-other:
		t.Fatalf("cancelled subscriber got %+v", c)
	case <-time.After(200 * time.Millisecond):
	}
}
