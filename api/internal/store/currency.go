package store

import (
	"context"
	"fmt"
)

// RelabelCurrency changes the currency of the Group ledgerID and of every
// Expense and Settlement in it, deleted or not, keeping the same numbers: it
// relabels, never converts. Checking that no amount loses decimals is the
// caller's job. It returns sql.ErrNoRows if ledgerID is not a live Group.
func (q *Queries) RelabelCurrency(ctx context.Context, ledgerID, currency string, version int64) error {
	if _, err := q.SetGroupCurrency(ctx, SetGroupCurrencyParams{ID: ledgerID, Currency: currency, Version: version}); err != nil {
		return err
	}
	if err := q.RelabelExpenses(ctx, RelabelExpensesParams{LedgerID: ledgerID, Currency: currency, Version: version}); err != nil {
		return fmt.Errorf("relabel expenses of %s: %w", ledgerID, err)
	}
	if err := q.RelabelSettlements(ctx, RelabelSettlementsParams{LedgerID: ledgerID, Currency: currency, Version: version}); err != nil {
		return fmt.Errorf("relabel settlements of %s: %w", ledgerID, err)
	}
	return nil
}
