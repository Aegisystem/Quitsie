package domain

import "fmt"

// ValidateExpense checks an Expense before it is stored in ledger. It does not
// recompute the OwedShares from the SplitMethod: the client did (ADR 0003).
func ValidateExpense(ledger Ledger, e Expense) error {
	if err := ledger.checkRecord(e.Currency); err != nil {
		return err
	}
	if e.Total <= 0 {
		return fmt.Errorf("total %d: %w", e.Total, ErrNonPositiveAmount)
	}
	if len(e.PaidShares) == 0 || len(e.OwedShares) == 0 {
		return ErrMissingShares
	}

	payers := make([]string, len(e.PaidShares))
	paid := make([]int64, len(e.PaidShares))
	for i, s := range e.PaidShares {
		payers[i], paid[i] = s.MemberID, s.Amount
	}
	if err := addUpTo(e.Total, paid, 1); err != nil {
		return fmt.Errorf("paid shares: %w", err)
	}
	if err := checkParticipants(ledger, payers); err != nil {
		return fmt.Errorf("paid shares: %w", err)
	}

	debtors := make([]string, len(e.OwedShares))
	owed := make([]int64, len(e.OwedShares))
	for i, s := range e.OwedShares {
		debtors[i], owed[i] = s.MemberID, s.Amount
	}
	// An OwedShare may round down to zero (1 cent split in two), and is kept
	// so the split survives re-editing (docs/reglas-de-negocio.md).
	if err := addUpTo(e.Total, owed, 0); err != nil {
		return fmt.Errorf("owed shares: %w", err)
	}
	if err := checkParticipants(ledger, debtors); err != nil {
		return fmt.Errorf("owed shares: %w", err)
	}
	return checkSplitValues(e.SplitMethod, e.OwedShares)
}

// checkSplitValues checks the format of the SplitMethod's inputs kept to
// re-edit the Expense. It never recomputes the OwedShares from them.
//
// Percentages have up to two decimals, so they are hundredths adding up to
// 10000, and weights are positive integers (docs/reglas-de-negocio.md). Every
// OwedShare of a percentage, weights or adjustment split carries its value, and
// equal and exact splits carry none.
func checkSplitValues(method SplitMethod, shares []OwedShare) error {
	switch method {
	case SplitPercentage:
		// Percentages are in hundredths, so they add up to 10000 (100%).
		var sum int64
		for _, s := range shares {
			if s.SplitValue == nil || *s.SplitValue <= 0 || *s.SplitValue > fullPercentage {
				return fmt.Errorf("member %q: percentage must be between 0.01%% and 100%%: %w", s.MemberID, ErrInvalidSplitValues)
			}
			sum += *s.SplitValue
		}
		if sum != fullPercentage {
			return fmt.Errorf("percentages add up to %d hundredths, not %d: %w", sum, fullPercentage, ErrInvalidSplitValues)
		}
		return nil
	case SplitWeights:
		for _, s := range shares {
			if s.SplitValue == nil || *s.SplitValue <= 0 {
				return fmt.Errorf("member %q: weight must be a positive integer: %w", s.MemberID, ErrInvalidSplitValues)
			}
		}
		return nil
	case SplitAdjustment:
		// An adjustment is an amount in minor units and may be zero or negative.
		for _, s := range shares {
			if s.SplitValue == nil {
				return fmt.Errorf("member %q: missing adjustment: %w", s.MemberID, ErrInvalidSplitValues)
			}
		}
		return nil
	case SplitEqual, SplitExact:
		for _, s := range shares {
			if s.SplitValue != nil {
				return fmt.Errorf("member %q: %s split takes no values: %w", s.MemberID, method, ErrInvalidSplitValues)
			}
		}
		return nil
	default:
		return fmt.Errorf("%w: %q", ErrUnknownSplitMethod, method)
	}
}

// fullPercentage is 100% in hundredths of a percent.
const fullPercentage = 10000

// checkParticipants checks that each Participant is an active Member of ledger
// and appears only once.
func checkParticipants(ledger Ledger, memberIDs []string) error {
	seen := make(map[string]bool, len(memberIDs))
	for _, id := range memberIDs {
		if seen[id] {
			return fmt.Errorf("member %q: %w", id, ErrDuplicateParticipant)
		}
		seen[id] = true
		if !ledger.isActiveMember(id) {
			return fmt.Errorf("member %q: %w", id, ErrNotActiveMember)
		}
	}
	return nil
}

// addUpTo checks that every amount is at least `least` and that they add up to total.
func addUpTo(total int64, amounts []int64, least int64) error {
	for _, a := range amounts {
		if a < least {
			return fmt.Errorf("%d: %w", a, ErrNonPositiveAmount)
		}
	}
	var sum int64
	for _, a := range amounts {
		// Checking before adding keeps sum <= total, so it never overflows.
		if a > total-sum {
			return fmt.Errorf("add up to more than the total %d: %w", total, ErrSharesDoNotAddUp)
		}
		sum += a
	}
	if sum != total {
		return fmt.Errorf("add up to %d, total is %d: %w", sum, total, ErrSharesDoNotAddUp)
	}
	return nil
}
