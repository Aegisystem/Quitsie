package store

import (
	"context"
	"fmt"
)

// LedgerRecords are all the Expenses, with their shares, and all the
// Settlements of a ledger, deleted ones included.
type LedgerRecords struct {
	Expenses    []ExpenseWithShares
	Settlements []Settlement
}

// GetLedgerRecords returns every Expense and Settlement of the ledger, in a
// handful of queries rather than one per Expense.
func (q *Queries) GetLedgerRecords(ctx context.Context, ledgerID string) (LedgerRecords, error) {
	expenses, err := q.ListLedgerExpenses(ctx, ledgerID)
	if err != nil {
		return LedgerRecords{}, fmt.Errorf("list expenses of %s: %w", ledgerID, err)
	}
	paid, err := q.ListLedgerPaidShares(ctx, ledgerID)
	if err != nil {
		return LedgerRecords{}, fmt.Errorf("list paid shares of %s: %w", ledgerID, err)
	}
	owed, err := q.ListLedgerOwedShares(ctx, ledgerID)
	if err != nil {
		return LedgerRecords{}, fmt.Errorf("list owed shares of %s: %w", ledgerID, err)
	}
	settlements, err := q.ListLedgerSettlements(ctx, ledgerID)
	if err != nil {
		return LedgerRecords{}, fmt.Errorf("list settlements of %s: %w", ledgerID, err)
	}

	records := LedgerRecords{Expenses: make([]ExpenseWithShares, len(expenses)), Settlements: settlements}
	index := make(map[string]int, len(expenses))
	for i, e := range expenses {
		records.Expenses[i] = ExpenseWithShares{Expense: e}
		index[e.ID] = i
	}
	for _, s := range paid {
		i := index[s.ExpenseID]
		records.Expenses[i].PaidShares = append(records.Expenses[i].PaidShares, s)
	}
	for _, s := range owed {
		i := index[s.ExpenseID]
		records.Expenses[i].OwedShares = append(records.Expenses[i].OwedShares, s)
	}
	return records, nil
}
