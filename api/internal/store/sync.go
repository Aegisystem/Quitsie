package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
)

// Changes is what a User has to apply to their replica to catch up from a
// cursor (docs/arquitectura.md, "Traer cambios"). Deleted ledgers, Expenses
// and Settlements come with their deletion date, so the replica can remove
// them.
type Changes struct {
	// Cursor is the since of the next pull.
	Cursor int64
	// HasMore says the pull stopped at Cursor with more left to bring.
	HasMore     bool
	Ledgers     []Ledger
	Members     []Member
	Expenses    []ExpenseWithShares
	Settlements []Settlement
	Activities  []Activity
	// DroppedLedgerIDs are the ledgers where the User's Member became former
	// after the cursor. The User no longer sees them, so the replica drops
	// them with everything in them.
	DroppedLedgerIDs []string
	// RemovedLedgerIDs are those of DroppedLedgerIDs where another Member
	// removed the User, instead of the User leaving.
	RemovedLedgerIDs []string
}

// Read runs fn in a read-only transaction, so everything it reads comes from
// the same snapshot (BeginRead). It does not block the writer.
func Read(ctx context.Context, db *sql.DB, fn func(q *Queries) error) error {
	tx, err := BeginRead(ctx, db)
	if err != nil {
		return fmt.Errorf("begin read: %w", err)
	}
	defer tx.Rollback()
	return fn(New(tx))
}

// ChangesSince returns what changed after the cursor since in the ledgers
// where userID has an active Member, and the whole of any ledger where that
// Member changed after since, which is how a ledger the User just joined
// arrives complete. With since 0 it returns everything the User can see, in one
// go however much that is; the API pulls in pages with Queries.ChangesSince.
//
// It reads in a single transaction, so every row and the new cursor come from
// the same snapshot and the next pull never skips a change.
func ChangesSince(ctx context.Context, db *sql.DB, userID string, since int64) (Changes, error) {
	var c Changes
	err := Read(ctx, db, func(q *Queries) (err error) {
		c, err = q.ChangesSince(ctx, userID, since, math.MaxInt64)
		return err
	})
	return c, err
}

// ChangesSince is ChangesSince inside a Read, for a caller that needs to read
// more from the same snapshot. It stops after about maxRows rows and says so in
// HasMore: more when one write stamped many rows, which never get split, or when
// the User joined a ledger, which arrives whole. The ledgers to drop always come
// whole: there are few, and repeating them does no harm.
func (q *Queries) ChangesSince(ctx context.Context, userID string, since, maxRows int64) (Changes, error) {
	var c Changes
	var err error
	if c.Cursor, err = q.CurrentVersion(ctx); err != nil {
		return Changes{}, fmt.Errorf("current version: %w", err)
	}
	end, err := q.PullPageEnd(ctx, PullPageEndParams{UserID: userID, Since: since, MaxRows: maxRows})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		// Fewer rows than maxRows are left, so this pull takes them all.
	case err != nil:
		return Changes{}, fmt.Errorf("pull page end: %w", err)
	case end < c.Cursor:
		c.Cursor, c.HasMore = end, true
	}
	upto := c.Cursor
	if c.Ledgers, err = q.ChangedLedgers(ctx, ChangedLedgersParams{UserID: userID, Since: since, Upto: upto}); err != nil {
		return Changes{}, fmt.Errorf("changed ledgers: %w", err)
	}
	if c.Members, err = q.ChangedMembers(ctx, ChangedMembersParams{UserID: userID, Since: since, Upto: upto}); err != nil {
		return Changes{}, fmt.Errorf("changed members: %w", err)
	}
	if c.Expenses, err = pullExpenses(ctx, q, userID, since, upto); err != nil {
		return Changes{}, err
	}
	if c.Settlements, err = q.ChangedSettlements(ctx, ChangedSettlementsParams{UserID: userID, Since: since, Upto: upto}); err != nil {
		return Changes{}, fmt.Errorf("changed settlements: %w", err)
	}
	if c.Activities, err = q.ChangedActivities(ctx, ChangedActivitiesParams{UserID: userID, Since: since, Upto: upto}); err != nil {
		return Changes{}, fmt.Errorf("changed activities: %w", err)
	}
	if c.DroppedLedgerIDs, err = q.DroppedLedgers(ctx, DroppedLedgersParams{UserID: userID, Since: since}); err != nil {
		return Changes{}, fmt.Errorf("dropped ledgers: %w", err)
	}
	if c.RemovedLedgerIDs, err = q.RemovedFromLedgers(ctx, RemovedFromLedgersParams{UserID: userID, Since: since}); err != nil {
		return Changes{}, fmt.Errorf("removed from ledgers: %w", err)
	}
	return c, nil
}

func pullExpenses(ctx context.Context, q *Queries, userID string, since, upto int64) ([]ExpenseWithShares, error) {
	expenses, err := q.ChangedExpenses(ctx, ChangedExpensesParams{UserID: userID, Since: since, Upto: upto})
	if err != nil {
		return nil, fmt.Errorf("changed expenses: %w", err)
	}
	paid, err := q.ChangedPaidShares(ctx, ChangedPaidSharesParams{UserID: userID, Since: since, Upto: upto})
	if err != nil {
		return nil, fmt.Errorf("changed paid shares: %w", err)
	}
	owed, err := q.ChangedOwedShares(ctx, ChangedOwedSharesParams{UserID: userID, Since: since, Upto: upto})
	if err != nil {
		return nil, fmt.Errorf("changed owed shares: %w", err)
	}

	out := make([]ExpenseWithShares, len(expenses))
	byID := make(map[string]*ExpenseWithShares, len(expenses))
	for i, e := range expenses {
		out[i] = ExpenseWithShares{Expense: e, PaidShares: []PaidShare{}, OwedShares: []OwedShare{}}
		byID[e.ID] = &out[i]
	}
	for _, s := range paid {
		byID[s.ExpenseID].PaidShares = append(byID[s.ExpenseID].PaidShares, s)
	}
	for _, s := range owed {
		byID[s.ExpenseID].OwedShares = append(byID[s.ExpenseID].OwedShares, s)
	}
	return out, nil
}
