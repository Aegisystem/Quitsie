package httpapi

import (
	"context"
	"encoding/json"
	"errors"
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
var unauthenticated = recordProblem(http.StatusUnauthorized, ProblemCodeUnauthenticated, "Sign in first.", "")

func recordProblem(status int, code ProblemCode, title, detail string) Problem {
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
		p := recordProblem(http.StatusBadRequest, ProblemCodeValidationFailed, "The request does not match the API contract.",
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

func (s *Server) PullChanges(ctx context.Context, req PullChangesRequestObject) (PullChangesResponseObject, error) {
	user, ok := auth.CurrentUser(ctx)
	if !ok {
		return PullChanges401ApplicationProblemPlusJSONResponse{UnauthorizedApplicationProblemPlusJSONResponse(unauthenticated)}, nil
	}
	if req.Params.Since < 0 {
		p := recordProblem(http.StatusBadRequest, ProblemCodeValidationFailed, "The request does not match the API contract.", "since cannot be negative")
		return PullChanges400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(p)}, nil
	}
	c, err := sync.Pull(ctx, s.deps.DB, user.ID, req.Params.Since)
	if errors.Is(err, sync.ErrCursorAhead) {
		return PullChanges410ApplicationProblemPlusJSONResponse(recordProblem(http.StatusGone, ProblemCodeCursorInvalid, "The cursor is ahead of the server.", "")), nil
	}
	if err != nil {
		return nil, err
	}
	out, err := pulledChanges(c)
	if err != nil {
		return nil, err
	}
	return PullChanges200JSONResponse(out), nil
}

func pulledChanges(c sync.Changes) (Changes, error) {
	out := Changes{
		Cursor:           c.Cursor,
		Groups:           []Group{},
		Friendships:      []Friendship{},
		Members:          make([]Member, len(c.Members)),
		Expenses:         make([]Expense, len(c.Expenses)),
		Settlements:      make([]Settlement, len(c.Settlements)),
		Activities:       make([]Activity, len(c.Activities)),
		Debts:            make([]LedgerDebts, len(c.Debts)),
		DroppedLedgers:   make([]DroppedLedger, len(c.Dropped)),
		VisibleLedgerIds: make([]openapi_types.UUID, len(c.VisibleLedgerIDs)),
	}
	for i, l := range c.Debts {
		debts := make([]Debt, len(l.Debts))
		for j, d := range l.Debts {
			debts[j] = Debt{FromMemberId: toUUID(d.FromMemberID), ToMemberId: toUUID(d.ToMemberID), Amount: d.Amount, Currency: string(d.Currency)}
		}
		out.Debts[i] = LedgerDebts{LedgerId: toUUID(l.LedgerID), Debts: debts}
	}
	for i, d := range c.Dropped {
		out.DroppedLedgers[i] = DroppedLedger{LedgerId: toUUID(d.LedgerID), Reason: DroppedLedgerReason(d.Reason)}
	}
	for i, ledgerID := range c.VisibleLedgerIDs {
		out.VisibleLedgerIds[i] = toUUID(ledgerID)
	}
	for _, l := range c.Ledgers {
		if l.Kind == "group" {
			out.Groups = append(out.Groups, Group{
				Id: toUUID(l.ID), Name: l.Name, Currency: *l.Currency, SimplifyDebts: l.SimplifyDebts,
				CreatedAt: l.CreatedAt, Version: l.Version,
			})
			continue
		}
		out.Friendships = append(out.Friendships, Friendship{Id: toUUID(l.ID), CreatedAt: l.CreatedAt, Version: l.Version})
	}
	for i, m := range c.Members {
		out.Members[i] = pulledMember(m)
	}
	for i, e := range c.Expenses {
		out.Expenses[i] = pulledExpense(e)
	}
	for i, st := range c.Settlements {
		out.Settlements[i] = pulledSettlement(st)
	}
	for i, a := range c.Activities {
		var details map[string]any
		if err := json.Unmarshal([]byte(a.Details), &details); err != nil {
			return Changes{}, fmt.Errorf("details of activity %s: %w", a.ID, err)
		}
		out.Activities[i] = Activity{
			Id: toUUID(a.ID), LedgerId: toUUID(a.LedgerID), UserId: toUUID(a.UserID), Kind: ActivityKind(a.Kind),
			Details: details, CreatedAt: a.CreatedAt, Version: a.Version,
		}
	}
	return out, nil
}

func pulledMember(m store.Member) Member {
	out := Member{Id: toUUID(m.ID), LedgerId: toUUID(m.LedgerID), Name: m.Name, Status: MemberStatus(m.Status), Version: m.Version}
	if m.UserID != nil {
		userID := toUUID(*m.UserID)
		out.UserId = &userID
		return out
	}
	// Only a pending Member shows the email given when adding them; a linked
	// User's is private.
	out.Email = (*openapi_types.Email)(m.Email)
	return out
}

func pulledExpense(e store.ExpenseWithShares) Expense {
	out := Expense{
		Id:          toUUID(e.ID),
		LedgerId:    toUUID(e.LedgerID),
		Description: e.Description,
		Date:        openapi_types.Date{Time: e.Date},
		Currency:    e.Currency,
		Total:       e.Total,
		Category:    (*Category)(e.Category),
		SplitMethod: SplitMethod(e.SplitMethod),
		PaidShares:  make([]PaidShare, len(e.PaidShares)),
		OwedShares:  make([]OwedShare, len(e.OwedShares)),
		CreatedBy:   toUUID(e.CreatedBy),
		CreatedAt:   e.CreatedAt,
		Version:     e.Version,
		DeletedAt:   e.DeletedAt,
	}
	for i, p := range e.PaidShares {
		out.PaidShares[i] = PaidShare{MemberId: toUUID(p.MemberID), Amount: p.Amount}
	}
	for i, o := range e.OwedShares {
		out.OwedShares[i] = OwedShare{MemberId: toUUID(o.MemberID), Amount: o.Amount, SplitValue: o.SplitValue}
	}
	return out
}

func pulledSettlement(st store.Settlement) Settlement {
	return Settlement{
		Id:           toUUID(st.ID),
		LedgerId:     toUUID(st.LedgerID),
		FromMemberId: toUUID(st.FromMemberID),
		ToMemberId:   toUUID(st.ToMemberID),
		Amount:       st.Amount,
		Currency:     st.Currency,
		Date:         openapi_types.Date{Time: st.Date},
		CreatedBy:    toUUID(st.CreatedBy),
		CreatedAt:    st.CreatedAt,
		Version:      st.Version,
		DeletedAt:    st.DeletedAt,
	}
}

// toUUID turns an id from the store into the contract's type. Every id in the
// database is a UUID: the server makes them, or the contract checked them.
func toUUID(id string) openapi_types.UUID {
	return uuid.MustParse(id)
}
