package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Activity kinds of records; the values are the contract's.
const (
	activityExpenseCreated     = "expense_created"
	activityExpenseUpdated     = "expense_updated"
	activityExpenseDeleted     = "expense_deleted"
	activityExpenseRestored    = "expense_restored"
	activitySettlementCreated  = "settlement_created"
	activitySettlementDeleted  = "settlement_deleted"
	activitySettlementRestored = "settlement_restored"
)

// logActivity records who changed what in a ledger, in the write of the change.
func logActivity(ctx context.Context, q *store.Queries, ledgerID, userID, kind string, details map[string]any, version int64, now time.Time) error {
	raw, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("activity %s: %w", kind, err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("activity %s: %w", kind, err)
	}
	err = q.CreateActivity(ctx, store.CreateActivityParams{
		ID:        id.String(),
		LedgerID:  ledgerID,
		UserID:    userID,
		Kind:      kind,
		Details:   string(raw),
		Version:   version,
		CreatedAt: now,
	})
	if err != nil {
		return fmt.Errorf("activity %s: %w", kind, err)
	}
	return nil
}

// expenseSummary is what the Activity of an Expense shows about it.
func expenseSummary(e store.Expense) map[string]any {
	return map[string]any{"description": e.Description, "total": e.Total, "currency": e.Currency}
}

func expenseDetails(e store.Expense) map[string]any {
	details := expenseSummary(e)
	details["expenseId"] = e.ID
	return details
}

func settlementDetails(s store.Settlement) map[string]any {
	return map[string]any{
		"settlementId": s.ID,
		"fromMemberId": s.FromMemberID,
		"toMemberId":   s.ToMemberID,
		"amount":       s.Amount,
		"currency":     s.Currency,
	}
}
