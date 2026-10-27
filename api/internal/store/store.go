// Package store owns the SQLite database: opening it, applying the goose
// migrations, and the typed queries that sqlc generates from queries/*.sql.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/Aegisystem/Quitsie/api/migrations"
)

// Open opens the SQLite database at path, creating it if needed.
func Open(ctx context.Context, path string) (*sql.DB, error) {
	q := url.Values{}
	// WAL lets readers work while the single writer writes, and is what
	// Litestream replicates to R2.
	q.Set("_journal_mode", "WAL")
	// NORMAL is durable enough with WAL and much faster than FULL.
	q.Set("_synchronous", "NORMAL")
	// SQLite ignores REFERENCES unless this is on for every connection.
	q.Set("_foreign_keys", "1")
	// Wait up to 5 s for the write lock instead of failing with SQLITE_BUSY.
	q.Set("_busy_timeout", "5000")
	// Take the write lock when a transaction begins, not halfway through it.
	q.Set("_txlock", "immediate")
	q.Set("_time_format", "sqlite")

	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	return db, nil
}

// Migrate applies every pending migration in api/migrations.
func Migrate(ctx context.Context, db *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
