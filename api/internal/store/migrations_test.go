package store_test

import (
	"context"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/Aegisystem/Quitsie/api/internal/store"
	"github.com/Aegisystem/Quitsie/api/migrations"
)

// Migration 00004 turns Debt simplification on in every Group that had it off,
// with a new version so that sync sends the Group, and its Debts, again.
func TestMigrationTurnsDebtSimplificationOn(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.DownTo(ctx, 3); err != nil {
		t.Fatal(err)
	}

	write(t, db, func(q *store.Queries, version int64) error {
		for _, id := range []string{"off", "on", "deleted"} {
			group := newGroup(id, version)
			group.SimplifyDebts = id == "on"
			if err := q.CreateLedger(ctx, group); err != nil {
				return err
			}
		}
		return nil
	})
	if _, err := db.ExecContext(ctx, `UPDATE ledgers SET deleted_at = now() WHERE id = 'deleted'`); err != nil {
		t.Fatal(err)
	}
	before, err := store.New(db).CurrentVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}

	after, err := store.New(db).CurrentVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]struct {
		simplify bool
		version  int64
	}{
		"off":     {true, after},
		"on":      {true, before},
		"deleted": {false, before},
	}
	for id, w := range want {
		var simplify bool
		var version int64
		if err := db.QueryRowContext(ctx, `SELECT simplify_debts, version FROM ledgers WHERE id = $1`, id).Scan(&simplify, &version); err != nil {
			t.Fatal(err)
		}
		if simplify != w.simplify || version != w.version {
			t.Errorf("%s: simplify_debts %v, version %d; want %v, %d", id, simplify, version, w.simplify, w.version)
		}
	}
	if after != before+1 {
		t.Errorf("the migration took versions %d to %d, want one", before, after)
	}
}
