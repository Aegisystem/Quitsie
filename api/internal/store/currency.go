package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

// RelabelCurrency changes the currency of the Group ledgerID and of every
// Expense and Settlement in it, deleted or not. It relabels, never converts:
// people keep reading the same numbers. Amounts are stored in minor units, so
// when the decimals differ the stored numbers change with them: 90.00 COP,
// stored as 9000, become 90 CLP, stored as 90. Checking first that no amount
// loses a decimal or overflows is the caller's job (domain.Relabel); here
// division truncates. It returns sql.ErrNoRows if ledgerID is not a live Group.
func (q *Queries) RelabelCurrency(ctx context.Context, ledgerID, currency string, version int64) error {
	ledger, err := q.GetLedger(ctx, ledgerID)
	if err != nil {
		return err
	}
	if ledger.Currency == nil {
		return sql.ErrNoRows
	}
	fromDecimals, ok := domain.Currency(*ledger.Currency).Decimals()
	if !ok {
		return fmt.Errorf("group %s has the unknown currency %q", ledgerID, *ledger.Currency)
	}
	toDecimals, ok := domain.Currency(currency).Decimals()
	if !ok {
		return fmt.Errorf("unknown currency %q", currency)
	}

	if _, err := q.SetGroupCurrency(ctx, SetGroupCurrencyParams{ID: ledgerID, Currency: currency, Version: version}); err != nil {
		return err
	}
	if err := q.RelabelExpenses(ctx, RelabelExpensesParams{LedgerID: ledgerID, Currency: currency, Version: version}); err != nil {
		return fmt.Errorf("relabel expenses of %s: %w", ledgerID, err)
	}
	if err := q.RelabelSettlements(ctx, RelabelSettlementsParams{LedgerID: ledgerID, Currency: currency, Version: version}); err != nil {
		return fmt.Errorf("relabel settlements of %s: %w", ledgerID, err)
	}
	return q.scaleLedgerAmounts(ctx, ledgerID, fromDecimals, toDecimals)
}

// scaleLedgerAmounts rewrites every amount of the ledger, deleted Expenses and
// Settlements included, from minor units with fromDecimals to minor units with
// toDecimals: totals, PaidShares, OwedShares, adjustments and Settlements.
func (q *Queries) scaleLedgerAmounts(ctx context.Context, ledgerID string, fromDecimals, toDecimals int) error {
	multiplier, divisor := int64(1), int64(1)
	for d := fromDecimals; d < toDecimals; d++ {
		multiplier *= 10
	}
	for d := toDecimals; d < fromDecimals; d++ {
		divisor *= 10
	}
	if multiplier == divisor {
		return nil
	}
	if err := q.ScaleLedgerExpenseTotals(ctx, ScaleLedgerExpenseTotalsParams{Multiplier: multiplier, Divisor: divisor, LedgerID: ledgerID}); err != nil {
		return fmt.Errorf("scale expenses of %s: %w", ledgerID, err)
	}
	if err := q.ScaleLedgerPaidShares(ctx, ScaleLedgerPaidSharesParams{Multiplier: multiplier, Divisor: divisor, LedgerID: ledgerID}); err != nil {
		return fmt.Errorf("scale paid shares of %s: %w", ledgerID, err)
	}
	if err := q.ScaleLedgerOwedShares(ctx, ScaleLedgerOwedSharesParams{Multiplier: multiplier, Divisor: divisor, LedgerID: ledgerID}); err != nil {
		return fmt.Errorf("scale owed shares of %s: %w", ledgerID, err)
	}
	if err := q.ScaleLedgerSettlements(ctx, ScaleLedgerSettlementsParams{Multiplier: multiplier, Divisor: divisor, LedgerID: ledgerID}); err != nil {
		return fmt.Errorf("scale settlements of %s: %w", ledgerID, err)
	}
	return nil
}
