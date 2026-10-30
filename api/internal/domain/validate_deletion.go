package domain

import "fmt"

// ValidateExpenseDeletion checks that e can be deleted from ledger. An Expense
// with a former member cannot: deleting it would change their Balance, which
// is zero, and nothing could put it right, since restoring the Expense or
// recording a Settlement takes active Members only.
func ValidateExpenseDeletion(ledger Ledger, e Expense) error {
	shares := sharesByMember(e)
	for _, m := range ledger.Members {
		if _, ok := shares[m.ID]; ok && m.Status == MemberFormer {
			return fmt.Errorf("member %q: %w", m.ID, ErrFormerMemberChanged)
		}
	}
	return nil
}

// ValidateSettlementDeletion checks that s can be deleted from ledger, for the
// same reason as ValidateExpenseDeletion.
func ValidateSettlementDeletion(ledger Ledger, s Settlement) error {
	for _, m := range ledger.Members {
		if (m.ID == s.FromMemberID || m.ID == s.ToMemberID) && m.Status == MemberFormer {
			return fmt.Errorf("member %q: %w", m.ID, ErrFormerMemberChanged)
		}
	}
	return nil
}
