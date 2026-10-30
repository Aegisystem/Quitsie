package httpapi

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/store"
	"github.com/Aegisystem/Quitsie/api/internal/sync"
)

// Handlers for Expenses, Settlements and sync. They translate between the
// contract and package sync, which holds the rules.

// maxBatch is the most Expenses, and the most Settlements, a RecordBatch takes.
const maxBatch = 100

// unauthenticated answers a request without a valid session.
var unauthenticated = problem(http.StatusUnauthorized, ProblemCodeUnauthenticated, "Sign in first.", "")

func problem(status int, code ProblemCode, title, detail string) Problem {
	p := Problem{Status: status, Code: code, Title: title}
	if detail != "" {
		p.Detail = &detail
	}
	return p
}

func (s *Server) PushRecords(ctx context.Context, req PushRecordsRequestObject) (PushRecordsResponseObject, error) {
	user, ok := auth.CurrentUser(ctx)
	if !ok {
		return PushRecords401ApplicationProblemPlusJSONResponse{UnauthorizedApplicationProblemPlusJSONResponse(unauthenticated)}, nil
	}
	if len(req.Body.Expenses) > maxBatch || len(req.Body.Settlements) > maxBatch {
		p := problem(http.StatusBadRequest, ProblemCodeValidationFailed, "The request does not match the API contract.",
			fmt.Sprintf("a batch takes at most %d expenses and %d settlements", maxBatch, maxBatch))
		return PushRecords400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	}
	batch := sync.Batch{}
	for _, e := range req.Body.Expenses {
		batch.Expenses = append(batch.Expenses, newExpense(e))
	}
	for _, st := range req.Body.Settlements {
		batch.Settlements = append(batch.Settlements, store.Settlement{
			ID:           st.Id.String(),
			LedgerID:     st.LedgerId.String(),
			FromMemberID: st.FromMemberId.String(),
			ToMemberID:   st.ToMemberId.String(),
			Amount:       st.Amount,
			Currency:     st.Currency,
			Date:         st.Date.Time,
		})
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
