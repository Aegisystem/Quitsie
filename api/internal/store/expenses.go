package store

import (
	"context"
	"fmt"
)

// ExpenseWithShares is an Expense with its PaidShares and OwedShares, which
// travel with it and have no version of their own.
type ExpenseWithShares struct {
	Expense
	PaidShares []PaidShare
	OwedShares []OwedShare
}

// CreateExpense stores e and its shares, stamped with e.Version, and returns
// what was stored. The shares' ExpenseID is taken from e.ID. The id comes from
// the client, so if it already exists CreateExpense changes nothing and returns
// the stored Expense with created set to false; the caller decides whether a
// resent Expense is acceptable.
func (q *Queries) CreateExpense(ctx context.Context, e ExpenseWithShares) (stored ExpenseWithShares, created bool, err error) {
	inserted, err := q.InsertExpense(ctx, InsertExpenseParams{
		ID:          e.ID,
		LedgerID:    e.LedgerID,
		Description: e.Description,
		Date:        e.Date,
		Currency:    e.Currency,
		Total:       e.Total,
		Category:    e.Category,
		SplitMethod: e.SplitMethod,
		CreatedBy:   e.CreatedBy,
		Version:     e.Version,
		CreatedAt:   e.CreatedAt,
	})
	if err != nil {
		return ExpenseWithShares{}, false, fmt.Errorf("insert expense %s: %w", e.ID, err)
	}
	if inserted > 0 {
		if err := q.insertShares(ctx, e); err != nil {
			return ExpenseWithShares{}, false, err
		}
	}
	stored, err = q.GetExpenseWithShares(ctx, e.ID)
	return stored, inserted > 0, err
}

func (q *Queries) insertShares(ctx context.Context, e ExpenseWithShares) error {
	for _, s := range e.PaidShares {
		if err := q.InsertPaidShare(ctx, InsertPaidShareParams{ExpenseID: e.ID, MemberID: s.MemberID, Amount: s.Amount}); err != nil {
			return fmt.Errorf("insert paid share of %s for %s: %w", e.ID, s.MemberID, err)
		}
	}
	for _, s := range e.OwedShares {
		err := q.InsertOwedShare(ctx, InsertOwedShareParams{ExpenseID: e.ID, MemberID: s.MemberID, Amount: s.Amount, SplitValue: s.SplitValue})
		if err != nil {
			return fmt.Errorf("insert owed share of %s for %s: %w", e.ID, s.MemberID, err)
		}
	}
	return nil
}

// GetExpenseWithShares returns the Expense id, deleted or not, with its shares.
func (q *Queries) GetExpenseWithShares(ctx context.Context, id string) (ExpenseWithShares, error) {
	e, err := q.GetExpense(ctx, id)
	if err != nil {
		return ExpenseWithShares{}, err
	}
	paid, err := q.ListPaidSharesOfExpense(ctx, id)
	if err != nil {
		return ExpenseWithShares{}, fmt.Errorf("list paid shares of %s: %w", id, err)
	}
	owed, err := q.ListOwedSharesOfExpense(ctx, id)
	if err != nil {
		return ExpenseWithShares{}, fmt.Errorf("list owed shares of %s: %w", id, err)
	}
	return ExpenseWithShares{Expense: e, PaidShares: paid, OwedShares: owed}, nil
}
