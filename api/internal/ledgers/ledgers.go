// Package ledgers loads a ledger from the store in domain terms, so the
// handlers that check Balances, validate records or relabel the currency share
// one mapping between store rows and domain values.
package ledgers

import (
	"context"
	"fmt"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Snapshot is a ledger with its Members and all its records, deleted ones
// included, as read in one go.
type Snapshot struct {
	Ledger  store.Ledger
	Members []store.Member
	Records store.LedgerRecords
}

// Load reads the ledger ledgerID with its Members and records. Call it inside
// the same transaction as the write that depends on it.
func Load(ctx context.Context, q *store.Queries, ledgerID string) (Snapshot, error) {
	ledger, err := q.GetLedger(ctx, ledgerID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("get ledger %s: %w", ledgerID, err)
	}
	members, err := q.ListMembers(ctx, ledgerID)
	if err != nil {
		return Snapshot{}, fmt.Errorf("list members of %s: %w", ledgerID, err)
	}
	records, err := q.GetLedgerRecords(ctx, ledgerID)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Ledger: ledger, Members: members, Records: records}, nil
}

// Domain is the ledger as the domain validates records against it.
func (s Snapshot) Domain() domain.Ledger {
	return Ledger(s.Ledger, s.Members)
}

// Expenses are all the ledger's Expenses in domain terms, deleted ones included.
func (s Snapshot) Expenses() []domain.Expense {
	out := make([]domain.Expense, len(s.Records.Expenses))
	for i, e := range s.Records.Expenses {
		out[i] = Expense(e)
	}
	return out
}

// Settlements are all the ledger's Settlements in domain terms, deleted ones included.
func (s Snapshot) Settlements() []domain.Settlement {
	out := make([]domain.Settlement, len(s.Records.Settlements))
	for i, st := range s.Records.Settlements {
		out[i] = Settlement(st)
	}
	return out
}

// Balances are the Members' Balances per currency; deleted records do not count.
func (s Snapshot) Balances() []domain.Balance {
	return domain.Balances(s.Expenses(), s.Settlements())
}

// Ledger maps a stored ledger and its Members to the domain.
func Ledger(l store.Ledger, members []store.Member) domain.Ledger {
	d := domain.Ledger{Kind: domain.LedgerKind(l.Kind), Members: make([]domain.Member, len(members))}
	if l.Currency != nil {
		d.Currency = domain.Currency(*l.Currency)
	}
	for i, m := range members {
		d.Members[i] = domain.Member{ID: m.ID, Status: domain.MemberStatus(m.Status)}
	}
	return d
}

// Expense maps a stored Expense and its shares to the domain.
func Expense(e store.ExpenseWithShares) domain.Expense {
	d := domain.Expense{
		ID:          e.ID,
		Currency:    domain.Currency(e.Currency),
		Total:       e.Total,
		SplitMethod: domain.SplitMethod(e.SplitMethod),
		PaidShares:  make([]domain.PaidShare, len(e.PaidShares)),
		OwedShares:  make([]domain.OwedShare, len(e.OwedShares)),
		DeletedAt:   e.DeletedAt,
	}
	for i, s := range e.PaidShares {
		d.PaidShares[i] = domain.PaidShare{MemberID: s.MemberID, Amount: s.Amount}
	}
	for i, s := range e.OwedShares {
		d.OwedShares[i] = domain.OwedShare{MemberID: s.MemberID, Amount: s.Amount, SplitValue: s.SplitValue}
	}
	return d
}

// Settlement maps a stored Settlement to the domain.
func Settlement(s store.Settlement) domain.Settlement {
	return domain.Settlement{
		ID:           s.ID,
		FromMemberID: s.FromMemberID,
		ToMemberID:   s.ToMemberID,
		Amount:       s.Amount,
		Currency:     domain.Currency(s.Currency),
		DeletedAt:    s.DeletedAt,
	}
}
