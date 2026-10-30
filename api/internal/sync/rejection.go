package sync

import (
	"errors"
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Reason is why a record was rejected. The values are the contract's.
type Reason string

const (
	// ReasonLedgerUnavailable: the ledger was deleted, or the User is not an
	// active Member of it.
	ReasonLedgerUnavailable Reason = "ledger_unavailable"
	// ReasonCurrencyMismatch: the Group currency is not the one the client knew.
	ReasonCurrencyMismatch Reason = "currency_mismatch"
	// ReasonInvalidParticipants: someone is not an active Member, or a former
	// member's shares changed.
	ReasonInvalidParticipants Reason = "invalid_participants"
	// ReasonInvalidAmounts: an amount is out of range or the shares do not add up.
	ReasonInvalidAmounts Reason = "invalid_amounts"
	// ReasonIDConflict: the id belongs to another record.
	ReasonIDConflict Reason = "id_conflict"
	// ReasonInvalidRecord: any other rule.
	ReasonInvalidRecord Reason = "invalid_record"
)

// Rejection is a record that breaks a rule, and which one.
type Rejection struct {
	Reason Reason
	Err    error
}

func (r *Rejection) Error() string { return string(r.Reason) + ": " + r.Err.Error() }
func (r *Rejection) Unwrap() error { return r.Err }

// rejection tells why the domain rejected a record.
func rejection(err error) *Rejection {
	switch {
	case errors.Is(err, domain.ErrNotGroupCurrency):
		return &Rejection{ReasonCurrencyMismatch, err}
	case errors.Is(err, domain.ErrNotActiveMember), errors.Is(err, domain.ErrDuplicateParticipant),
		errors.Is(err, domain.ErrSameMember), errors.Is(err, domain.ErrFormerMemberChanged):
		return &Rejection{ReasonInvalidParticipants, err}
	case errors.Is(err, domain.ErrNonPositiveAmount), errors.Is(err, domain.ErrAmountTooLarge),
		errors.Is(err, domain.ErrSharesDoNotAddUp), errors.Is(err, domain.ErrMissingShares):
		return &Rejection{ReasonInvalidAmounts, err}
	default:
		return &Rejection{ReasonInvalidRecord, err}
	}
}

// categories are the Categories of docs/reglas-de-negocio.md, as the contract names them.
var categories = []string{
	"food", "groceries", "transport", "lodging", "housing", "utilities",
	"entertainment", "shopping", "health", "gifts", "other",
}

// checkExpense checks what the domain does not: the description and the
// Category, as the contract restricts them.
func checkExpense(e store.ExpenseWithShares) error {
	if n := utf8.RuneCountInString(e.Description); n < 1 || n > 200 {
		return fmt.Errorf("description has %d characters, want 1 to 200", n)
	}
	if e.Category != nil && !slices.Contains(categories, *e.Category) {
		return fmt.Errorf("unknown category %q", *e.Category)
	}
	return nil
}
