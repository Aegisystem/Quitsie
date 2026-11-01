package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
)

// The attack of finding 13 in docs/revision-seguridad.md: thousands of
// Expenses of the largest amount, all one way, used to wrap the Balances
// around and turn the Debts the other way.
func TestPushRecordsStopsABalanceAtTheLimit(t *testing.T) {
	a := newRecordsAPI(t)
	ana, beto := a.signIn("ana@example.com"), a.signIn("beto@example.com")
	f := a.seedFriendship(ana, beto)
	maxLunch := func() httpapi.NewExpense {
		e := f.lunch()
		e.Id, e.Total = id(uuid.NewString()), domain.MaxAmount
		e.PaidShares[0].Amount, e.OwedShares[0].Amount = domain.MaxAmount, domain.MaxAmount
		return e
	}

	var results []httpapi.RecordResult
	for range 30 {
		batch := expenses()
		for range 100 {
			batch.Expenses = append(batch.Expenses, maxLunch())
		}
		results = append(results, a.push(t, ana, batch)...)
	}

	// Nine fit: 9 × 10^15 is under 2^53 - 1, and ten are not.
	for i, r := range results {
		if i < 9 && r.Status != httpapi.Accepted {
			t.Fatalf("result %d = %+v, want accepted", i, r)
		}
		if i >= 9 && (r.Status != httpapi.Rejected || r.Reason == nil || *r.Reason != httpapi.RecordResultReasonInvalidAmounts) {
			t.Fatalf("result %d = %+v, want rejected with invalid_amounts", i, r)
		}
	}
	pulled := a.pull(t, beto, 0)
	if len(pulled.Expenses) != 9 {
		t.Errorf("pulled %d expenses, want the 9 accepted", len(pulled.Expenses))
	}
	want := []httpapi.Debt{{FromMemberId: id(f.bM), ToMemberId: id(f.aM), Amount: 9 * domain.MaxAmount, Currency: "JPY"}}
	if got := debtsOf(t, pulled, f.id); len(got) != 1 || got[0] != want[0] {
		t.Errorf("debts = %+v, want Beto owing Ana %+v", got, want)
	}
}

// nineMax pushes, as Ana, nine Expenses of the largest amount that she paid
// for Beto, which leave her a Balance of 9 × 10^15, close to the limit.
func (a *recordsAPI) nineMax(t *testing.T, tr trip) []httpapi.NewExpense {
	t.Helper()
	var nine []httpapi.NewExpense
	for range 9 {
		nine = append(nine, tr.expenseBy(tr.anaM, tr.betoM, domain.MaxAmount))
	}
	for _, r := range a.push(t, tr.ana, expenses(nine...)) {
		if r.Status != httpapi.Accepted {
			t.Fatalf("push = %+v, want accepted", r)
		}
	}
	return nine
}

func TestUpdateExpenseRefusesToTakeABalancePastTheLimit(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	a.nineMax(t, tr)
	taxi := tr.expenseBy(tr.anaM, tr.betoM, 100)
	version := a.pushed(t, tr.ana, taxi)
	bigger := tr.expenseBy(tr.anaM, tr.betoM, domain.MaxAmount)

	rec := a.update(tr.ana, taxi.Id, editOf(bigger, version))

	decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeInvalidAmounts)
	for _, e := range a.pull(t, tr.ana, 0).Expenses {
		if e.Id == taxi.Id && e.Total != 100 {
			t.Errorf("taxi = %+v, want it unchanged", e)
		}
	}
}

func TestRestoreExpenseRefusesToTakeABalancePastTheLimit(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	first := a.nineMax(t, tr)[0]
	a.do(tr.ana, http.MethodDelete, "/api/v1/expenses/"+first.Id.String(), nil)
	a.pushed(t, tr.ana, tr.expenseBy(tr.anaM, tr.betoM, domain.MaxAmount))

	rec := a.do(tr.ana, http.MethodPost, "/api/v1/expenses/"+first.Id.String()+"/restore", nil)

	decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeInvalidAmounts)
	for _, e := range a.pull(t, tr.ana, 0).Expenses {
		if e.Id == first.Id && e.DeletedAt == nil {
			t.Errorf("expense = %+v, want it still deleted", e)
		}
	}
}

func TestDeleteExpenseRefusesToTakeABalancePastTheLimit(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	a.nineMax(t, tr)
	// Beto pays one back for Ana, and Ana one more for Beto: she is at 9 × 10^15 again.
	back := tr.expenseBy(tr.betoM, tr.anaM, domain.MaxAmount)
	a.pushed(t, tr.ana, back)
	a.pushed(t, tr.ana, tr.expenseBy(tr.anaM, tr.betoM, domain.MaxAmount))

	rec := a.do(tr.ana, http.MethodDelete, "/api/v1/expenses/"+back.Id.String(), nil)

	decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeInvalidAmounts)
	for _, e := range a.pull(t, tr.ana, 0).Expenses {
		if e.Id == back.Id && e.DeletedAt != nil {
			t.Errorf("expense = %+v, want it still counting", e)
		}
	}
}

func TestDeleteAndRestoreSettlementRefuseToTakeABalancePastTheLimit(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	a.nineMax(t, tr)
	settlement := func(from, to string) httpapi.NewSettlement {
		s := tr.payback()
		s.FromMemberId, s.ToMemberId, s.Amount = id(from), id(to), domain.MaxAmount
		if r := a.push(t, tr.ana, settlements(s))[0]; r.Status != httpapi.Accepted {
			t.Fatalf("push = %+v, want accepted", r)
		}
		return s
	}

	// Beto pays Ana back 10^15 and she lends him as much again: deleting the
	// payback would leave her at 10^16.
	payback := settlement(tr.betoM, tr.anaM)
	a.pushed(t, tr.ana, tr.expenseBy(tr.anaM, tr.betoM, domain.MaxAmount))

	decodeProblem(t, a.do(tr.ana, http.MethodDelete, "/api/v1/settlements/"+payback.Id.String(), nil), http.StatusBadRequest, httpapi.ProblemCodeInvalidAmounts)

	// Beto lends Ana 10^15, she pays it back, and it is deleted. Once she lends
	// him as much again, restoring it would leave her at 10^16.
	a.pushed(t, tr.ana, tr.expenseBy(tr.betoM, tr.anaM, domain.MaxAmount))
	refund := settlement(tr.anaM, tr.betoM)
	if rec := a.do(tr.ana, http.MethodDelete, "/api/v1/settlements/"+refund.Id.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d (%s), want 204", rec.Code, rec.Body)
	}
	a.pushed(t, tr.ana, tr.expenseBy(tr.anaM, tr.betoM, domain.MaxAmount))

	decodeProblem(t, a.do(tr.ana, http.MethodPost, "/api/v1/settlements/"+refund.Id.String()+"/restore", nil), http.StatusBadRequest, httpapi.ProblemCodeInvalidAmounts)
}

func TestUpdateGroupRefusesACurrencyThatTakesABalancePastTheLimit(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	// 10^15 cents in ten Expenses: as fils (KWD has three decimals) each one
	// is 10^15, which fits, but Ana's Balance is 10^16, which does not.
	var ten []httpapi.NewExpense
	for range 10 {
		ten = append(ten, tr.expenseBy(tr.anaM, tr.betoM, domain.MaxAmount/10))
	}
	a.push(t, tr.ana, expenses(ten...))

	rec := a.do(tr.ana, http.MethodPatch, "/api/v1/groups/"+tr.id, map[string]string{"currency": "KWD"})

	decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeInvalidAmounts)
	if got := a.pull(t, tr.ana, 0).Groups[0].Currency; got != "USD" {
		t.Errorf("currency = %s, want USD", got)
	}
}
