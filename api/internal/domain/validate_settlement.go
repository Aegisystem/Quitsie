package domain

import "fmt"

// ValidateSettlement checks a Settlement before it is stored in ledger.
func ValidateSettlement(ledger Ledger, s Settlement) error {
	if err := ledger.checkRecord(s.Currency); err != nil {
		return err
	}
	if s.Amount <= 0 {
		return fmt.Errorf("amount %d: %w", s.Amount, ErrNonPositiveAmount)
	}
	if s.FromMemberID == s.ToMemberID {
		return fmt.Errorf("member %q: %w", s.FromMemberID, ErrSameMember)
	}
	// Like a new Expense, a Settlement involves active Members only: a former
	// member left the Group with a zero Balance, and a Settlement would change it.
	for _, id := range []string{s.FromMemberID, s.ToMemberID} {
		if !ledger.isActiveMember(id) {
			return fmt.Errorf("member %q: %w", id, ErrNotActiveMember)
		}
	}
	return nil
}
