package domain

import (
	"fmt"
	"strings"
)

// RelabelError lists the Expenses and Settlements with an amount that cannot be
// written in the new currency without losing decimals (10.50 USD in JPY) or
// going over MaxAmount, so the Group knows which ones to correct before
// changing its currency.
type RelabelError struct {
	To            Currency
	ExpenseIDs    []string
	SettlementIDs []string
}

func (e *RelabelError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "cannot relabel to %s without losing decimals or going over 10^15:", e.To)
	if len(e.ExpenseIDs) > 0 {
		fmt.Fprintf(&b, " expenses %s", strings.Join(e.ExpenseIDs, ", "))
	}
	if len(e.SettlementIDs) > 0 {
		fmt.Fprintf(&b, " settlements %s", strings.Join(e.SettlementIDs, ", "))
	}
	return b.String()
}

// Relabel moves Expenses and Settlements to the currency to when the Group
// currency changes. It keeps the same numbers and never applies an exchange
// rate: 10.50 USD becomes 10.50 EUR, and 10.00 USD becomes 10 JPY. When some
// amount would lose decimals or go over MaxAmount it changes nothing and
// returns a *RelabelError.
// The inputs are never modified.
func Relabel(expenses []Expense, settlements []Settlement, to Currency) ([]Expense, []Settlement, error) {
	toDecimals, ok := to.Decimals()
	if !ok {
		return nil, nil, fmt.Errorf("%w: %q", ErrUnknownCurrency, to)
	}
	relabelErr := &RelabelError{To: to}

	relabeledExpenses := make([]Expense, len(expenses))
	for i, e := range expenses {
		fromDecimals, ok := e.Currency.Decimals()
		if !ok {
			return nil, nil, fmt.Errorf("expense %s: %w: %q", e.ID, ErrUnknownCurrency, e.Currency)
		}
		exact := true
		amount := func(a int64) int64 {
			relabeled, ok := relabelAmount(a, fromDecimals, toDecimals)
			exact = exact && ok
			return relabeled
		}

		e.Currency = to
		e.Total = amount(e.Total)
		e.PaidShares = append([]PaidShare(nil), e.PaidShares...)
		for j := range e.PaidShares {
			e.PaidShares[j].Amount = amount(e.PaidShares[j].Amount)
		}
		e.OwedShares = append([]OwedShare(nil), e.OwedShares...)
		for j := range e.OwedShares {
			share := &e.OwedShares[j]
			share.Amount = amount(share.Amount)
			// Adjustments are amounts too; percentages and weights have no currency.
			if e.SplitMethod == SplitAdjustment && share.SplitValue != nil {
				adjustment := amount(*share.SplitValue)
				share.SplitValue = &adjustment
			}
		}
		if !exact {
			relabelErr.ExpenseIDs = append(relabelErr.ExpenseIDs, e.ID)
		}
		relabeledExpenses[i] = e
	}

	relabeledSettlements := make([]Settlement, len(settlements))
	for i, s := range settlements {
		fromDecimals, ok := s.Currency.Decimals()
		if !ok {
			return nil, nil, fmt.Errorf("settlement %s: %w: %q", s.ID, ErrUnknownCurrency, s.Currency)
		}
		amount, ok := relabelAmount(s.Amount, fromDecimals, toDecimals)
		if !ok {
			relabelErr.SettlementIDs = append(relabelErr.SettlementIDs, s.ID)
		}
		s.Currency = to
		s.Amount = amount
		relabeledSettlements[i] = s
	}

	if len(relabelErr.ExpenseIDs) > 0 || len(relabelErr.SettlementIDs) > 0 {
		return nil, nil, relabelErr
	}
	return relabeledExpenses, relabeledSettlements, nil
}

// relabelAmount writes an amount in minor units with fromDecimals as the same
// number in minor units with toDecimals. It reports false when that would drop
// a non-zero decimal or go over MaxAmount, either way.
func relabelAmount(amount int64, fromDecimals, toDecimals int) (int64, bool) {
	for d := fromDecimals; d > toDecimals; d-- {
		if amount%10 != 0 {
			return 0, false
		}
		amount /= 10
	}
	for d := fromDecimals; d < toDecimals; d++ {
		// MaxAmount is a multiple of 10, so this keeps amount within it.
		if amount > MaxAmount/10 || amount < -MaxAmount/10 {
			return 0, false
		}
		amount *= 10
	}
	return amount, amount <= MaxAmount && amount >= -MaxAmount
}
