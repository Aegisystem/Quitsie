package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

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
	Activities  []store.Activity
	// VisibleLedgerIDs are all the ledgers the User can see now, changed or
	// not. The replica drops any other.
	VisibleLedgerIDs []string
}

// ErrCursorAhead means the cursor is over the last version the server handed
// out, for example after restoring a backup, so the device rebuilds its replica.
var ErrCursorAhead = errors.New("the cursor is ahead of the server")

// Pull returns what changed after the cursor since for userID, all from one
// snapshot. With since 0 it returns everything the User can see.
func Pull(ctx context.Context, db *sql.DB, userID string, since int64) (Changes, error) {
	var out Changes
	err := store.Read(ctx, db, func(q *store.Queries) error {
		c, err := q.ChangesSince(ctx, userID, since)
		if err != nil {
			return err
		}
		if since > c.Cursor {
			return ErrCursorAhead
		}
		out = Changes{Cursor: c.Cursor, Ledgers: c.Ledgers, Members: c.Members, Expenses: c.Expenses, Settlements: c.Settlements, Activities: c.Activities}
		visible, err := q.ListLedgersOfUser(ctx, userID)
		if err != nil {
			return fmt.Errorf("list ledgers of user: %w", err)
		}
		out.VisibleLedgerIDs = make([]string, len(visible))
		for i, l := range visible {
			out.VisibleLedgerIDs[i] = l.ID
		}
		return nil
	})
	return out, err
}
