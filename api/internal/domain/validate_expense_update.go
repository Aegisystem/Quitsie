package domain

import "fmt"

// ValidateExpenseUpdate checks an edit of the stored Expense before it replaces
// it. The rules of ValidateExpense apply, except that a former member who
// takes part in stored may stay, as long as their PaidShare and OwedShare do
// not change: they left with a zero Balance (docs/reglas-de-negocio.md).
func ValidateExpenseUpdate(ledger Ledger, stored, updated Expense) error {
	before, after := sharesByMember(stored), sharesByMember(updated)
	kept := make(map[string]bool)
	for _, m := range ledger.Members {
		if m.Status != MemberFormer {
			continue
		}
		if before[m.ID] != after[m.ID] {
			return fmt.Errorf("member %q: %w", m.ID, ErrFormerMemberChanged)
		}
		kept[m.ID] = true
	}
	return ValidateExpense(ledger.keeping(kept), updated)
}

// keeping returns a copy of l in which the given Members count as active.
func (l Ledger) keeping(ids map[string]bool) Ledger {
	members := make([]Member, len(l.Members))
	for i, m := range l.Members {
		if ids[m.ID] {
			m.Status = MemberActive
		}
		members[i] = m
	}
	l.Members = members
	return l
}

// memberShares is what a Participant paid and owes of an Expense, and
// whether they have a PaidShare and an OwedShare at all: an OwedShare of zero
// still counts as taking part.
type memberShares struct {
	paid, owed int64
	pays, owes bool
}

func sharesByMember(e Expense) map[string]memberShares {
	shares := make(map[string]memberShares)
	for _, s := range e.PaidShares {
		m := shares[s.MemberID]
		m.paid, m.pays = s.Amount, true
		shares[s.MemberID] = m
	}
	for _, s := range e.OwedShares {
		m := shares[s.MemberID]
		m.owed, m.owes = s.Amount, true
		shares[s.MemberID] = m
	}
	return shares
}
