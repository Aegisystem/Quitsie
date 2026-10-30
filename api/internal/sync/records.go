package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/ledgers"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// ErrNotFound means the record does not exist or the User cannot see it,
// which the API does not tell apart.
var ErrNotFound = errors.New("no such record")

// UpdateExpense replaces the contents of the Expense e.ID, which the client
// read at expectedVersion, on behalf of userID, and returns it as stored. Only
// e's description, date, currency, total, Category, SplitMethod and shares
// count: its ledger, author and creation time never change.
//
// It returns ErrNotFound, store.ErrVersionConflict if the Expense changed or
// was deleted after expectedVersion, or a *Rejection if the edit breaks a rule.
func UpdateExpense(ctx context.Context, db *sql.DB, userID string, e store.ExpenseWithShares, expectedVersion int64) (store.ExpenseWithShares, error) {
	var updated store.ExpenseWithShares
	_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
		stored, ledger, err := expenseOf(ctx, q, userID, e.ID)
		if err != nil {
			return err
		}
		if stored.Version != expectedVersion || stored.DeletedAt != nil {
			return store.ErrVersionConflict
		}
		if err := checkExpense(e); err != nil {
			return &Rejection{ReasonInvalidRecord, err}
		}
		if err := domain.ValidateExpenseUpdate(ledger.Domain(), ledgers.Expense(stored), ledgers.Expense(e)); err != nil {
			return rejection(err)
		}

		e.Version = version
		if updated, err = q.UpdateExpense(ctx, e, expectedVersion); err != nil {
			return err
		}
		details := map[string]any{
			"expenseId": e.ID,
			"before":    expenseSummary(stored.Expense),
			"after":     expenseSummary(updated.Expense),
		}
		return logActivity(ctx, q, stored.LedgerID, userID, activityExpenseUpdated, details, version, time.Now().UTC())
	})
	if err != nil {
		return store.ExpenseWithShares{}, err
	}
	return updated, nil
}

// errUnchanged rolls back the write of an operation that finds its work
// already done, so repeating it spends no version and leaves no Activity.
var errUnchanged = errors.New("already done")

// write runs fn with store.Write; errUnchanged from fn is not a failure.
func write(ctx context.Context, db *sql.DB, fn func(q *store.Queries, version int64) error) error {
	_, err := store.Write(ctx, db, fn)
	if errors.Is(err, errUnchanged) {
		return nil
	}
	return err
}

// DeleteExpense makes the Expense id a deleted expense, which no longer counts
// in Balances. Deleting it again changes nothing. It returns ErrNotFound if
// userID cannot see it, or a *Rejection if it includes a former member.
func DeleteExpense(ctx context.Context, db *sql.DB, userID, id string) error {
	return write(ctx, db, func(q *store.Queries, version int64) error {
		stored, ledger, err := expenseOf(ctx, q, userID, id)
		if err != nil {
			return err
		}
		if stored.DeletedAt != nil {
			return errUnchanged
		}
		if err := domain.ValidateExpenseDeletion(ledger.Domain(), ledgers.Expense(stored)); err != nil {
			return rejection(err)
		}
		now := time.Now().UTC()
		if _, err := q.DeleteExpense(ctx, store.DeleteExpenseParams{DeletedAt: &now, Version: version, ID: id}); err != nil {
			return fmt.Errorf("delete expense %s: %w", id, err)
		}
		return logActivity(ctx, q, stored.LedgerID, userID, activityExpenseDeleted, expenseDetails(stored.Expense), version, now)
	})
}

// RestoreExpense makes a deleted Expense count again, with no time limit, and
// returns it; a live one comes back as it is. It returns ErrNotFound, or a
// *Rejection if the Expense includes someone who is now a former member.
func RestoreExpense(ctx context.Context, db *sql.DB, userID, id string) (store.ExpenseWithShares, error) {
	var restored store.ExpenseWithShares
	err := write(ctx, db, func(q *store.Queries, version int64) error {
		stored, ledger, err := expenseOf(ctx, q, userID, id)
		if err != nil {
			return err
		}
		restored = stored
		if stored.DeletedAt == nil {
			return errUnchanged
		}
		if err := domain.ValidateExpense(ledger.Domain(), ledgers.Expense(stored)); err != nil {
			return rejection(err)
		}
		if _, err := q.RestoreExpense(ctx, store.RestoreExpenseParams{Version: version, ID: id}); err != nil {
			return fmt.Errorf("restore expense %s: %w", id, err)
		}
		restored.Version, restored.DeletedAt = version, nil
		return logActivity(ctx, q, stored.LedgerID, userID, activityExpenseRestored, expenseDetails(stored.Expense), version, time.Now().UTC())
	})
	if err != nil {
		return store.ExpenseWithShares{}, err
	}
	return restored, nil
}

// expenseOf loads the Expense id and its ledger, or returns ErrNotFound if
// userID cannot use that ledger.
func expenseOf(ctx context.Context, q *store.Queries, userID, id string) (store.ExpenseWithShares, *ledgers.Snapshot, error) {
	e, err := q.GetExpenseWithShares(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.ExpenseWithShares{}, nil, ErrNotFound
	}
	if err != nil {
		return store.ExpenseWithShares{}, nil, fmt.Errorf("get expense %s: %w", id, err)
	}
	ledger, err := usableLedger(ctx, q, userID, e.LedgerID)
	if err != nil {
		return store.ExpenseWithShares{}, nil, err
	}
	if ledger == nil {
		return store.ExpenseWithShares{}, nil, ErrNotFound
	}
	return e, ledger, nil
}

// DeleteSettlement makes the Settlement id stop counting in Balances. Deleting
// it again changes nothing. It returns ErrNotFound if userID cannot see it, or
// a *Rejection if one of the two people is a former member.
func DeleteSettlement(ctx context.Context, db *sql.DB, userID, id string) error {
	return write(ctx, db, func(q *store.Queries, version int64) error {
		stored, ledger, err := settlementOf(ctx, q, userID, id)
		if err != nil {
			return err
		}
		if stored.DeletedAt != nil {
			return errUnchanged
		}
		if err := domain.ValidateSettlementDeletion(ledger.Domain(), ledgers.Settlement(stored)); err != nil {
			return rejection(err)
		}
		now := time.Now().UTC()
		if _, err := q.DeleteSettlement(ctx, store.DeleteSettlementParams{DeletedAt: &now, Version: version, ID: id}); err != nil {
			return fmt.Errorf("delete settlement %s: %w", id, err)
		}
		return logActivity(ctx, q, stored.LedgerID, userID, activitySettlementDeleted, settlementDetails(stored), version, now)
	})
}

// RestoreSettlement makes a deleted Settlement count again and returns it; a
// live one comes back as it is. It returns ErrNotFound, or a *Rejection if
// one of the two people is now a former member.
func RestoreSettlement(ctx context.Context, db *sql.DB, userID, id string) (store.Settlement, error) {
	var restored store.Settlement
	err := write(ctx, db, func(q *store.Queries, version int64) error {
		stored, ledger, err := settlementOf(ctx, q, userID, id)
		if err != nil {
			return err
		}
		restored = stored
		if stored.DeletedAt == nil {
			return errUnchanged
		}
		if err := domain.ValidateSettlement(ledger.Domain(), ledgers.Settlement(stored)); err != nil {
			return rejection(err)
		}
		if restored, err = q.RestoreSettlement(ctx, store.RestoreSettlementParams{Version: version, ID: id}); err != nil {
			return fmt.Errorf("restore settlement %s: %w", id, err)
		}
		return logActivity(ctx, q, stored.LedgerID, userID, activitySettlementRestored, settlementDetails(stored), version, time.Now().UTC())
	})
	if err != nil {
		return store.Settlement{}, err
	}
	return restored, nil
}

// settlementOf loads the Settlement id and its ledger, or returns ErrNotFound
// if userID cannot use that ledger.
func settlementOf(ctx context.Context, q *store.Queries, userID, id string) (store.Settlement, *ledgers.Snapshot, error) {
	s, err := q.GetSettlement(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return store.Settlement{}, nil, ErrNotFound
	}
	if err != nil {
		return store.Settlement{}, nil, fmt.Errorf("get settlement %s: %w", id, err)
	}
	ledger, err := usableLedger(ctx, q, userID, s.LedgerID)
	if err != nil {
		return store.Settlement{}, nil, err
	}
	if ledger == nil {
		return store.Settlement{}, nil, ErrNotFound
	}
	return s, ledger, nil
}
