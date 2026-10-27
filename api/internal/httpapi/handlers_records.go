package httpapi

import (
	"context"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/store"
	"github.com/Aegisystem/Quitsie/api/internal/sync"
)

// Handlers for Expenses, Settlements and sync. They translate between the
// contract and package sync, which holds the rules.

func (s *Server) PushRecords(ctx context.Context, req PushRecordsRequestObject) (PushRecordsResponseObject, error) {
	user, _ := auth.CurrentUser(ctx)
	batch := sync.Batch{}
	for _, e := range req.Body.Expenses {
		batch.Expenses = append(batch.Expenses, newExpense(e))
	}
	results, err := sync.Push(ctx, s.deps.DB, user.ID, batch)
	if err != nil {
		return nil, err
	}
	out := PushRecords200JSONResponse{Results: make([]RecordResult, len(results))}
	for i, r := range results {
		out.Results[i] = recordResult(r)
	}
	return out, nil
}

func recordResult(r sync.Result) RecordResult {
	out := RecordResult{Id: toUUID(r.ID), Kind: RecordResultKind(r.Kind)}
	if r.Rejection != nil {
		reason, detail := RecordResultReason(r.Rejection.Reason), r.Rejection.Err.Error()
		out.Status, out.Reason, out.Detail = Rejected, &reason, &detail
		return out
	}
	version := r.Version
	out.Status, out.Version = Accepted, &version
	return out
}

func newExpense(e NewExpense) store.ExpenseWithShares {
	out := store.ExpenseWithShares{
		Expense: store.Expense{
			ID:          e.Id.String(),
			LedgerID:    e.LedgerId.String(),
			Description: e.Description,
			Date:        e.Date.Time,
			Currency:    e.Currency,
			Total:       e.Total,
			Category:    (*string)(e.Category),
			SplitMethod: string(e.SplitMethod),
		},
		PaidShares: make([]store.PaidShare, len(e.PaidShares)),
		OwedShares: make([]store.OwedShare, len(e.OwedShares)),
	}
	for i, s := range e.PaidShares {
		out.PaidShares[i] = store.PaidShare{ExpenseID: out.ID, MemberID: s.MemberId.String(), Amount: s.Amount}
	}
	for i, s := range e.OwedShares {
		out.OwedShares[i] = store.OwedShare{ExpenseID: out.ID, MemberID: s.MemberId.String(), Amount: s.Amount, SplitValue: s.SplitValue}
	}
	return out
}

// toUUID turns an id from the store into the contract's type. Every id in the
// database is a UUID: the server makes them, or the contract checked them.
func toUUID(id string) openapi_types.UUID {
	return uuid.MustParse(id)
}
