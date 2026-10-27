package domain

import "errors"

// Errors returned by this package, wrapped with details. Check them with
// errors.Is to tell the client why a record was rejected.
var (
	// ErrUnknownCurrency: the code is not a current ISO 4217 currency.
	ErrUnknownCurrency = errors.New("unknown currency")
)
