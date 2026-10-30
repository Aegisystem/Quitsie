package domain

import "errors"

// Errors returned by this package, wrapped with details. Check them with
// errors.Is to tell the client why a record was rejected.
var (
	// ErrUnknownCurrency: the code is not a current ISO 4217 currency.
	ErrUnknownCurrency = errors.New("unknown currency")
	// ErrNonPositiveAmount: a total, PaidShare or settlement amount is zero or
	// negative, or an OwedShare is negative. An OwedShare may be zero.
	ErrNonPositiveAmount = errors.New("amount must be greater than zero")
	// ErrAmountTooLarge: an amount is over MaxAmount.
	ErrAmountTooLarge = errors.New("amount is over 10^15 minor units")
	// ErrMissingShares: an Expense has no PaidShare or no OwedShare.
	ErrMissingShares = errors.New("an expense needs at least one paid share and one owed share")
	// ErrSharesDoNotAddUp: the PaidShares or the OwedShares do not add up to the total.
	ErrSharesDoNotAddUp = errors.New("shares do not add up to the total")
	// ErrDuplicateParticipant: a Participant has two PaidShares or two OwedShares.
	ErrDuplicateParticipant = errors.New("participant appears twice")
	// ErrNotActiveMember: a Participant is not an active Member of the ledger.
	ErrNotActiveMember = errors.New("not an active member of the ledger")
	// ErrFormerMemberChanged: an edit changes the PaidShare or OwedShare of a
	// former member, or a deletion removes a record that includes one. Their
	// Balance must stay at zero.
	ErrFormerMemberChanged = errors.New("the balance of a former member cannot change")
	// ErrSameMember: a Settlement goes from a Member to themselves.
	ErrSameMember = errors.New("a settlement needs two different members")
	// ErrNotGroupCurrency: a record of a Group is not in the Group currency.
	ErrNotGroupCurrency = errors.New("currency is not the group currency")
	// ErrFriendshipNotTwoMembers: a Friendship's ledger does not have exactly two Members (ADR 0004).
	ErrFriendshipNotTwoMembers = errors.New("a friendship has exactly two members")
	// ErrUnknownLedgerKind: the ledger is neither a Group nor a Friendship.
	ErrUnknownLedgerKind = errors.New("unknown ledger kind")
	// ErrUnknownSplitMethod: the SplitMethod is not one of the five known ones.
	ErrUnknownSplitMethod = errors.New("unknown split method")
	// ErrInvalidSplitValues: the SplitMethod's inputs are malformed (for
	// example, percentages that do not add up to 100%).
	ErrInvalidSplitValues = errors.New("invalid split values")
)
