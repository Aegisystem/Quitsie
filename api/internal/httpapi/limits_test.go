package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// fullBatch is the largest batch the contract allows in a Group of size
// Members: 100 Expenses and 100 Settlements, every description at its longest
// in four-byte characters, and every Member both paying and owing.
func (a *recordsAPI) fullBatch(tr trip, size int) httpapi.RecordBatch {
	a.t.Helper()
	members := []string{tr.anaM, tr.betoM, tr.carlaM}
	a.write(func(q *store.Queries, version int64) error {
		for len(members) < size {
			m := uuid.NewString()
			err := q.CreateMember(context.Background(), store.CreateMemberParams{
				ID: m, LedgerID: tr.id, Name: fmt.Sprintf("Miembro %d", len(members)), Version: version, CreatedAt: time.Now(),
			})
			if err != nil {
				return err
			}
			members = append(members, m)
		}
		return nil
	})

	const share = int64(20_000_000_000_000)
	percentage := int64(10000 / size)
	batch := httpapi.RecordBatch{}
	for range 100 {
		e := httpapi.NewExpense{
			Id: id(uuid.NewString()), LedgerId: id(tr.id), Description: strings.Repeat("🍕", 200), Date: day("2026-10-25"),
			Currency: "USD", Total: share * int64(size), SplitMethod: httpapi.Percentage,
		}
		for _, m := range members {
			e.PaidShares = append(e.PaidShares, httpapi.PaidShare{MemberId: id(m), Amount: share})
			e.OwedShares = append(e.OwedShares, httpapi.OwedShare{MemberId: id(m), Amount: share, SplitValue: &percentage})
		}
		batch.Expenses = append(batch.Expenses, e)
		batch.Settlements = append(batch.Settlements, httpapi.NewSettlement{
			Id: id(uuid.NewString()), LedgerId: id(tr.id), FromMemberId: id(tr.anaM), ToMemberId: id(tr.betoM),
			Amount: 1_000_000_000_000_000, Currency: "USD", Date: day("2026-10-25"),
		})
	}
	return batch
}

func TestTheLargestBatchFitsInTheBodyLimit(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	batch := a.fullBatch(tr, 50)

	results := a.push(t, tr.ana, batch)

	if len(results) != 200 {
		t.Fatalf("got %d results, want 200", len(results))
	}
	for _, r := range results {
		if r.Status != httpapi.Accepted {
			t.Fatalf("result = %+v, want accepted", r)
		}
	}
}

func TestABodyOverTheLimitGets413(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	const twoMiB = 2 << 20
	tooLarge := `{"name":"` + strings.Repeat("x", twoMiB) + `","currency":"COP"}`

	rec := ana.send(http.MethodPost, "/api/v1/groups", tooLarge)

	decodeProblem(t, rec, http.StatusRequestEntityTooLarge, httpapi.ProblemCodeBodyTooLarge)
	if changes := ana.pull(); len(changes.Ledgers) != 0 {
		t.Errorf("ledgers = %+v, want none", changes.Ledgers)
	}
}

func TestABodyJustUnderTheLimitIsRead(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	const twoMiB = 2 << 20
	prefix, suffix := `{"name":"`, `","currency":"COP"}`
	// Inside the limit, so what answers is the rule on names.
	fits := prefix + strings.Repeat("x", twoMiB-len(prefix)-len(suffix)) + suffix

	rec := ana.send(http.MethodPost, "/api/v1/groups", fits)

	decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
}
