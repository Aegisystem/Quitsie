package sync

import (
	"context"
	"database/sql"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Changes is what a device applies to its replica to catch up from a cursor.
type Changes struct {
	// Cursor is the since of the next pull.
	Cursor      int64
	Ledgers     []store.Ledger
	Members     []store.Member
	Expenses    []store.ExpenseWithShares
	Settlements []store.Settlement
}

// Pull returns what changed after the cursor since for userID, all from one
// snapshot. With since 0 it returns everything the User can see.
func Pull(ctx context.Context, db *sql.DB, userID string, since int64) (Changes, error) {
	var out Changes
	err := store.Read(ctx, db, func(q *store.Queries) error {
		c, err := q.ChangesSince(ctx, userID, since)
		if err != nil {
			return err
		}
		out = Changes{Cursor: c.Cursor, Ledgers: c.Ledgers, Members: c.Members, Expenses: c.Expenses, Settlements: c.Settlements}
		return nil
	})
	return out, err
}
