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
