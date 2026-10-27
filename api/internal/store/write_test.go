package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

func TestWriteStampsRowsWithTheNextVersion(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)

	for i, id := range []string{"ledger-1", "ledger-2"} {
		want := int64(i + 1)
		got, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
			return q.CreateLedger(ctx, newGroup(id, version))
		})
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("Write returned version %d, want %d", got, want)
		}
		ledger, err := store.New(db).GetLedger(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if ledger.Version != want {
			t.Errorf("%s has version %d, want %d", id, ledger.Version, want)
		}
	}
}

func TestWriteWritesNothingWhenFnFails(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	failure := errors.New("rule broken")

	_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, newGroup("ledger-1", version)); err != nil {
			return err
		}
		return failure
	})
	if !errors.Is(err, failure) {
		t.Fatalf("Write error = %v, want %v", err, failure)
	}

	if _, err := store.New(db).GetLedger(ctx, "ledger-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetLedger after a failed Write: err = %v, want sql.ErrNoRows", err)
	}
	next, err := store.Write(ctx, db, func(*store.Queries, int64) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if next != 1 {
		t.Errorf("version after a failed Write = %d, want 1 (the failed one is not spent)", next)
	}
}
