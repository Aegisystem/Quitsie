package store

import (
	"context"
	"database/sql"
	"fmt"
)

// Write runs fn in a write transaction and returns the version it was stamped
// with. The version is the next number of version_counter, taken before fn
// runs; fn puts it on every ledger, Member, Expense, Settlement and Activity it
// creates or changes, so sync can pull them (docs/arquitectura.md). If fn
// fails, nothing is written and the version is not spent.
func Write(ctx context.Context, db *sql.DB, fn func(q *Queries, version int64) error) (int64, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin write: %w", err)
	}
	defer tx.Rollback()

	q := New(tx)
	version, err := q.NextVersion(ctx)
	if err != nil {
		return 0, fmt.Errorf("next version: %w", err)
	}
	if err := fn(q, version); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit write: %w", err)
	}
	return version, nil
}
