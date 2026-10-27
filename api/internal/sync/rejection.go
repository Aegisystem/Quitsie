package sync

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
