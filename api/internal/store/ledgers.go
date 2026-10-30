package store

import (
	"context"
	"fmt"
)

// ScaleLedgerAmounts rewrites every amount of the ledger, deleted Expenses and
// Settlements included, from minor units with fromDecimals to minor units with
// toDecimals: totals, PaidShares, OwedShares, adjustments and Settlements.
// Relabeling keeps the number people read, so 10.00 USD stored as 1000 cents
// becomes 10 JPY stored as 10. Checking first that no amount loses a decimal
// or overflows is the caller's job (domain.Relabel); here division truncates.
func (q *Queries) ScaleLedgerAmounts(ctx context.Context, ledgerID string, fromDecimals, toDecimals int) error {
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
