package domain

import "time"

// Expense is something one or more Participants paid (PaidShares) and that is
// split among one or more Participants (OwedShares). Both add up to Total, in
// minor units of Currency.
type Expense struct {
	ID          string
	Currency    Currency
	Total       int64
	SplitMethod SplitMethod
	PaidShares  []PaidShare
	OwedShares  []OwedShare
	// DeletedAt is set on a deleted expense, which no longer counts in Balances.
	DeletedAt *time.Time
}

// PaidShare is the amount a Participant paid of an Expense (Aporte).
type PaidShare struct {
	MemberID string
	Amount   int64
}

// OwedShare is the amount a Participant must pay of an Expense (Cuota). The
// client computes it from the SplitMethod and the server stores it as is (ADR 0003).
type OwedShare struct {
	MemberID string
	Amount   int64
	// SplitValue is the Participant's input to the SplitMethod, kept to re-edit
	// the Expense: a percentage in hundredths (10000 = 100%), a weight, or an
	// adjustment in minor units. It is nil for SplitEqual and SplitExact.
	SplitValue *int64
}

// SplitMethod is the rule the client used to compute the OwedShares.
type SplitMethod string

const (
	SplitEqual      SplitMethod = "equal"
	SplitExact      SplitMethod = "exact"
	SplitPercentage SplitMethod = "percentage"
	SplitWeights    SplitMethod = "weights"
	SplitAdjustment SplitMethod = "adjustment"
)
