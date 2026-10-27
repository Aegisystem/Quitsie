package domain

import "time"

// Settlement records that FromMemberID paid ToMemberID back Amount, in minor
// units of Currency. It changes Balances but is not an Expense.
type Settlement struct {
	ID           string
	FromMemberID string
	ToMemberID   string
	Amount       int64
	Currency     Currency
	// DeletedAt is set on a deleted settlement, which no longer counts in Balances.
	DeletedAt *time.Time
}
