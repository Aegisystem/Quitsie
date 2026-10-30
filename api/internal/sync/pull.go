package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/ledgers"
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
	// Debts are the current Debts of every ledger that changed, simplified or
	// not as each ledger says. They replace the previous ones.
	Debts []LedgerDebts
	// Dropped are the ledgers the replica must drop with everything in them;
	// nothing else in Changes belongs to them.
	Dropped []DroppedLedger
	// VisibleLedgerIDs are all the ledgers the User can see now, changed or
	// not. The replica drops any other, even one that is not in Dropped.
	VisibleLedgerIDs []string
}

// LedgerDebts are the Debts of one ledger.
type LedgerDebts struct {
	LedgerID string
	Debts    []domain.Debt
}

// DropReason says why a ledger left the User's replica. The values are the
// contract's. Undoing a Claim has none, since the Member no longer points to
// the User: that ledger just stops being visible.
type DropReason string

const (
	DropDeleted DropReason = "deleted"
	DropLeft    DropReason = "left"
	DropRemoved DropReason = "removed"
)

// DroppedLedger is a ledger the User can no longer see.
type DroppedLedger struct {
	LedgerID string
	Reason   DropReason
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
		out = withoutDropped(c)
		if out.Debts, err = changedDebts(ctx, q, out); err != nil {
			return err
		}
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

// withoutDropped turns c into the pull's Changes, in which the ledgers the
// replica drops come only as Dropped.
func withoutDropped(c store.Changes) Changes {
	reasons := make(map[string]DropReason)
	for _, id := range c.DroppedLedgerIDs {
		reasons[id] = DropLeft
	}
	for _, id := range c.RemovedLedgerIDs {
		reasons[id] = DropRemoved
	}
	for _, l := range c.Ledgers {
		if l.DeletedAt != nil {
			reasons[l.ID] = DropDeleted
		}
	}
	gone := func(ledgerID string) bool {
		_, ok := reasons[ledgerID]
		return ok
	}

	out := Changes{
		Cursor:      c.Cursor,
		Ledgers:     slices.DeleteFunc(c.Ledgers, func(l store.Ledger) bool { return gone(l.ID) }),
		Members:     slices.DeleteFunc(c.Members, func(m store.Member) bool { return gone(m.LedgerID) }),
		Expenses:    slices.DeleteFunc(c.Expenses, func(e store.ExpenseWithShares) bool { return gone(e.LedgerID) }),
		Settlements: slices.DeleteFunc(c.Settlements, func(s store.Settlement) bool { return gone(s.LedgerID) }),
		Activities:  slices.DeleteFunc(c.Activities, func(a store.Activity) bool { return gone(a.LedgerID) }),
		Dropped:     make([]DroppedLedger, 0, len(reasons)),
	}
	for _, id := range slices.Sorted(maps.Keys(reasons)) {
		out.Dropped = append(out.Dropped, DroppedLedger{LedgerID: id, Reason: reasons[id]})
	}
	return out
}

// changedDebts computes the Debts of every ledger with a change that may move
// them. The client cannot: Debt simplification only runs here (ADR 0003).
func changedDebts(ctx context.Context, q *store.Queries, c Changes) ([]LedgerDebts, error) {
	changed := make(map[string]bool)
	for _, l := range c.Ledgers {
		changed[l.ID] = true
	}
	for _, m := range c.Members {
		changed[m.LedgerID] = true
	}
	for _, e := range c.Expenses {
		changed[e.LedgerID] = true
	}
	for _, s := range c.Settlements {
		changed[s.LedgerID] = true
	}

	out := make([]LedgerDebts, 0, len(changed))
	for _, id := range slices.Sorted(maps.Keys(changed)) {
		l, err := ledgers.Load(ctx, q, id)
		if err != nil {
			return nil, err
		}
		debts := domain.PairwiseDebts(l.Expenses(), l.Settlements())
		if l.Ledger.SimplifyDebts {
			debts = domain.SimplifyDebts(l.Balances())
		}
		out = append(out, LedgerDebts{LedgerID: id, Debts: debts})
	}
	return out, nil
}
