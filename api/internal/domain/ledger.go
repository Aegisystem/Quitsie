package domain

// Ledger holds the Expenses and Settlements of a Group or of a Friendship,
// which is a ledger of exactly two Members (ADR 0008).
type Ledger struct {
	Kind LedgerKind
	// Currency is the Group currency. A Friendship has none: each Expense carries its own.
	Currency Currency
	Members  []Member
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
