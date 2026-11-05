// Package store owns the PostgreSQL database: opening it, applying the goose
// migrations, and the typed queries that sqlc generates from queries/*.sql.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/Aegisystem/Quitsie/api/migrations"
)

// maxConns caps the connections the server holds. Writes run one at a time
// (BeginWrite), so more would only wait; small hosted plans allow few.
const maxConns = 10

// Open connects to the PostgreSQL database at url, such as
// postgres://user:password@host:5432/quitsie.
func Open(ctx context.Context, url string) (*sql.DB, error) {
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db := stdlib.OpenDB(*cfg, stdlib.OptionAfterConnect(func(_ context.Context, conn *pgx.Conn) error {
		// Instants come back in UTC, as the code writes them, instead of in
		// the server's local time zone.
		conn.TypeMap().RegisterType(&pgtype.Type{
			Name:  "timestamptz",
			OID:   pgtype.TimestamptzOID,
			Codec: &pgtype.TimestamptzCodec{ScanLocation: time.UTC},
		})
		return nil
	}))
	db.SetMaxOpenConns(maxConns)
	db.SetMaxIdleConns(maxConns)
	db.SetConnMaxIdleTime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("open database: %w", err)
	}
	return db, nil
}

// Migrate applies every pending migration in api/migrations.
func Migrate(ctx context.Context, db *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("load migrations: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// writeLock is the key of the advisory lock that BeginWrite takes.
const writeLock = 7_211_573_919 // any constant: nothing else takes advisory locks

// BeginWrite begins a transaction that writes, and waits until no other one
// is running. Writes run one after the other, as they did on SQLite, so a
// transaction that reads and then writes (a limit, a unique email, the next
// version) never races another one. Reads run alongside without waiting.
func BeginWrite(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, writeLock); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

// BeginRead begins a read-only transaction in which every query sees the
// same snapshot. PostgreSQL's default isolation takes a new snapshot for each
// query, and a pull could then pair rows from one moment with a cursor from
// another and skip a change.
func BeginRead(ctx context.Context, db *sql.DB) (*sql.Tx, error) {
	return db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
}
