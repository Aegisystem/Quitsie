package store

import (
	"context"
	"fmt"
)

// CreateSettlement stores s, stamped with s.Version, and returns what was
// stored. Like CreateExpense, if the client id already exists it changes
// nothing and returns the stored Settlement with created set to false.
func (q *Queries) CreateSettlement(ctx context.Context, s Settlement) (stored Settlement, created bool, err error) {
	inserted, err := q.InsertSettlement(ctx, InsertSettlementParams{
		ID:           s.ID,
		LedgerID:     s.LedgerID,
		FromMemberID: s.FromMemberID,
		ToMemberID:   s.ToMemberID,
		Amount:       s.Amount,
		Currency:     s.Currency,
		Date:         s.Date,
		CreatedBy:    s.CreatedBy,
		Version:      s.Version,
		CreatedAt:    s.CreatedAt,
	})
	if err != nil {
		return Settlement{}, false, fmt.Errorf("insert settlement %s: %w", s.ID, err)
	}
	stored, err = q.GetSettlement(ctx, s.ID)
	return stored, inserted > 0, err
}
