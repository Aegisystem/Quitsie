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
	if err := addUpTo(e.Total, paid); err != nil {
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
	if err := addUpTo(e.Total, owed); err != nil {
		return fmt.Errorf("owed shares: %w", err)
	}
	if err := checkParticipants(ledger, debtors); err != nil {
		return fmt.Errorf("owed shares: %w", err)
	}
	return nil
}

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

// addUpTo checks that every amount is positive and that they add up to total.
func addUpTo(total int64, amounts []int64) error {
	for _, a := range amounts {
		if a <= 0 {
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
