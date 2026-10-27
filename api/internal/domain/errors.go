package domain

import "errors"

// Errors returned by this package, wrapped with details. Check them with
// errors.Is to tell the client why a record was rejected.
var (
	// ErrUnknownCurrency: the code is not a current ISO 4217 currency.
	ErrUnknownCurrency = errors.New("unknown currency")
	// ErrNonPositiveAmount: a total, share or settlement amount is zero or negative.
	ErrNonPositiveAmount = errors.New("amount must be greater than zero")
	// ErrMissingShares: an Expense has no PaidShare or no OwedShare.
	ErrMissingShares = errors.New("an expense needs at least one paid share and one owed share")
	// ErrSharesDoNotAddUp: the PaidShares or the OwedShares do not add up to the total.
	ErrSharesDoNotAddUp = errors.New("shares do not add up to the total")
)
