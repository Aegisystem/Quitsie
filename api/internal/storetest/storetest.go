// Package storetest gives each test its own migrated database, so tests stay
// isolated and can run in parallel.
package storetest

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Open returns a new, empty, migrated database that is closed when t ends.
func Open(t testing.TB) *sql.DB {
	t.Helper()
	return open(t, filepath.Join(t.TempDir(), "quitsie.db"))
}

// Copy returns a new database with what db holds now, as a backup keeps it.
// db goes on working, and later writes to it do not reach the copy.
func Copy(t testing.TB, db *sql.DB) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "backup.db")
	if _, err := db.Exec(`VACUUM INTO ?`, path); err != nil {
		t.Fatal(err)
	}
	return open(t, path)
}

func open(t testing.TB, path string) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db
}
