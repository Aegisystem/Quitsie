package domain

import "fmt"

// Ledger holds the Expenses and Settlements of a Group or of a Friendship,
// which is a ledger of exactly two Members (ADR 0008).
type Ledger struct {
	Kind LedgerKind
	// Currency is the Group currency. A Friendship has none: each Expense carries its own.
	Currency Currency
	Members  []Member
}

// checkRecord checks the rules that depend on the kind of ledger for an Expense
// or Settlement in currency c. A Group's records are in the Group currency; a
// Friendship has exactly two Members and its records take any currency.
func (l Ledger) checkRecord(c Currency) error {
	if _, ok := c.Decimals(); !ok {
		return fmt.Errorf("%w: %q", ErrUnknownCurrency, c)
	}
	switch l.Kind {
	case LedgerGroup:
		if c != l.Currency {
			return fmt.Errorf("%s, group currency is %s: %w", c, l.Currency, ErrNotGroupCurrency)
		}
	case LedgerFriendship:
		if len(l.Members) != 2 {
			return fmt.Errorf("%d members: %w", len(l.Members), ErrFriendshipNotTwoMembers)
		}
	default:
		return fmt.Errorf("%w: %q", ErrUnknownLedgerKind, l.Kind)
	}
	return nil
}

func (l Ledger) isActiveMember(id string) bool {
	for _, m := range l.Members {
		if m.ID == id {
			return m.Status == MemberActive
		}
	}
	return false
}

// LedgerKind tells a Group's ledger from a Friendship's.
type LedgerKind string

const (
	LedgerGroup      LedgerKind = "group"
	LedgerFriendship LedgerKind = "friendship"
)

// Member is a person's place in a ledger. In a Friendship, the Friend is a Member too.
type Member struct {
	ID     string
	Status MemberStatus
}

// MemberStatus says whether a Member still takes part in new Expenses.
type MemberStatus string

const (
	MemberActive MemberStatus = "active"
	// MemberFormer is a former member: one who left the Group. They still
	// appear in past Expenses but take no part in new ones.
	MemberFormer MemberStatus = "former"
)
