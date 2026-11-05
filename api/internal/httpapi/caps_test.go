package httpapi_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
	"github.com/Aegisystem/Quitsie/api/internal/ledgers"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// fillLedger takes ledgerID to its limit of rows with Activities, the cheapest
// rows to make. Their ids are no UUIDs, so these ledgers cannot be pulled.
func fillLedger(t *testing.T, db *sql.DB, ledgerID, userID string) {
	t.Helper()
	_, err := db.Exec(`
		WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < $1)
		INSERT INTO activities (id, ledger_id, user_id, kind, details, version, created_at)
		SELECT 'filler-' || i, $2, $3, 'group_renamed', '{}', 0, '2026-10-31 00:00:00+00:00' FROM n`,
		ledgers.MaxRows, ledgerID, userID)
	if err != nil {
		t.Fatal(err)
	}
}

// addPendingMembers adds n pending Members to ledgerID directly.
func (w *world) addPendingMembers(ledgerID string, n int) {
	w.t.Helper()
	_, err := store.Write(context.Background(), w.db, func(q *store.Queries, version int64) error {
		for i := range n {
			err := q.CreateMember(context.Background(), store.CreateMemberParams{
				ID: uuid.NewString(), LedgerID: ledgerID, Name: fmt.Sprintf("Miembro %d", i), Version: version, CreatedAt: time.Now(),
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		w.t.Fatal(err)
	}
}

func TestAGroupHasAtMost200Members(t *testing.T) {
	w := newWorld(t)
	ana, beto, carla := w.signIn("Ana"), w.signIn("Beto"), w.signIn("Carla")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	token := ana.groupLink(groupID)
	beto.join(token)
	if rec := beto.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", ""); rec.Code != http.StatusNoContent {
		t.Fatalf("leave: code = %d", rec.Code)
	}
	w.addPendingMembers(groupID, 198) // with Ana and Beto, who stays as a former member

	decodeProblem(t, ana.send(http.MethodPost, "/api/v1/groups/"+groupID+"/members", `{"name":"Una más"}`), http.StatusConflict, httpapi.ProblemCodeLimitReached)
	decodeProblem(t, carla.send(http.MethodPost, "/api/v1/invites/"+token+"/join", ""), http.StatusConflict, httpapi.ProblemCodeLimitReached)

	// Coming back takes no new Member.
	if rec := beto.send(http.MethodPost, "/api/v1/invites/"+token+"/join", ""); rec.Code != http.StatusCreated {
		t.Errorf("former member coming back: code = %d, want 201 (body %s)", rec.Code, rec.Body)
	}
}

func TestAUserHoldsAtMost200Ledgers(t *testing.T) {
	w := newWorld(t)
	ana, beto := w.signIn("Ana"), w.signIn("Beto")
	theirGroup := beto.createGroup(`{"name":"Suyo","currency":"COP","pendingMembers":[{"name":"Ana"}]}`)
	token := beto.groupLink(theirGroup)
	for i := range 200 {
		ana.createGroup(fmt.Sprintf(`{"name":"Grupo %d","currency":"COP"}`, i))
	}

	decodeProblem(t, ana.send(http.MethodPost, "/api/v1/groups", `{"name":"Otro","currency":"COP"}`), http.StatusConflict, httpapi.ProblemCodeLimitReached)
	decodeProblem(t, ana.send(http.MethodPost, "/api/v1/friendships", `{"name":"Carla"}`), http.StatusConflict, httpapi.ProblemCodeLimitReached)
	decodeProblem(t, ana.send(http.MethodPost, "/api/v1/invites/"+token+"/join", ""), http.StatusConflict, httpapi.ProblemCodeLimitReached)
	decodeProblem(t, ana.claim(token, beto.memberID(theirGroup, "Ana")), http.StatusConflict, httpapi.ProblemCodeLimitReached)
}

func TestAFullLedgerTakesNoMoreRecords(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	kept, gone := tr.expenseBy(tr.anaM, tr.betoM, 1000), tr.expenseBy(tr.anaM, tr.betoM, 2000)
	settlement := httpapi.NewSettlement{
		Id: id(uuid.NewString()), LedgerId: id(tr.id), FromMemberId: id(tr.anaM), ToMemberId: id(tr.betoM),
		Amount: 500, Currency: "USD", Date: day("2026-10-25"),
	}
	keptVersion := a.pushed(t, tr.ana, kept)
	a.pushed(t, tr.ana, gone)
	a.push(t, tr.ana, httpapi.RecordBatch{Settlements: []httpapi.NewSettlement{settlement}})
	if rec := a.do(tr.ana, http.MethodDelete, "/api/v1/expenses/"+gone.Id.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: code = %d", rec.Code)
	}
	if rec := a.do(tr.ana, http.MethodDelete, "/api/v1/settlements/"+settlement.Id.String(), nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete settlement: code = %d", rec.Code)
	}
	fillLedger(t, a.db, tr.id, tr.ana.user.ID)

	t.Run("a new record is rejected", func(t *testing.T) {
		results := a.push(t, tr.ana, expenses(tr.expenseBy(tr.anaM, tr.betoM, 3000)))

		if r := results[0]; r.Status != httpapi.Rejected || r.Reason == nil || *r.Reason != httpapi.RecordResultReasonLimitReached {
			t.Errorf("result = %+v, want rejected with limit_reached", r)
		}
	})
	t.Run("an edit is refused", func(t *testing.T) {
		rec := a.update(tr.ana, kept.Id, httpapi.ExpenseUpdate{
			ExpectedVersion: keptVersion, Description: "Otra cosa", Date: kept.Date, Currency: "USD", Total: 1000,
			SplitMethod: kept.SplitMethod, PaidShares: kept.PaidShares, OwedShares: kept.OwedShares,
		})

		decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeLimitReached)
	})
	t.Run("restoring is refused", func(t *testing.T) {
		decodeProblem(t, a.do(tr.ana, http.MethodPost, "/api/v1/expenses/"+gone.Id.String()+"/restore", nil), http.StatusConflict, httpapi.ProblemCodeLimitReached)
		decodeProblem(t, a.do(tr.ana, http.MethodPost, "/api/v1/settlements/"+settlement.Id.String()+"/restore", nil), http.StatusConflict, httpapi.ProblemCodeLimitReached)
	})
	t.Run("deleting still works", func(t *testing.T) {
		if rec := a.do(tr.ana, http.MethodDelete, "/api/v1/expenses/"+kept.Id.String(), nil); rec.Code != http.StatusNoContent {
			t.Errorf("code = %d, want 204 (body %s)", rec.Code, rec.Body)
		}
	})
}

// Nobody can be trapped by a full Group, which any of its Members could cause.
func TestAFullGroupStillLetsPeopleOut(t *testing.T) {
	w := newWorld(t)
	ana, beto, carla := w.signIn("Ana"), w.signIn("Beto"), w.signIn("Carla")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	token := ana.groupLink(groupID)
	betoMember := beto.join(token).MemberId.String()
	carlaMember := carla.join(token).MemberId.String()
	fillLedger(t, w.db, groupID, ana.id)

	decodeProblem(t, ana.send(http.MethodPatch, "/api/v1/groups/"+groupID, `{"name":"Otro"}`), http.StatusConflict, httpapi.ProblemCodeLimitReached)
	decodeProblem(t, ana.send(http.MethodPost, "/api/v1/groups/"+groupID+"/members", `{"name":"Dani"}`), http.StatusConflict, httpapi.ProblemCodeLimitReached)
	decodeProblem(t, ana.send(http.MethodPost, "/api/v1/groups/"+groupID+"/invite-link", ""), http.StatusConflict, httpapi.ProblemCodeLimitReached)

	for _, out := range []struct {
		what, method, target string
		by                   *person
	}{
		{"undo a Claim", http.MethodDelete, "/api/v1/groups/" + groupID + "/members/" + betoMember + "/claim", ana},
		{"remove a Member", http.MethodDelete, "/api/v1/groups/" + groupID + "/members/" + carlaMember, ana},
		{"delete the Group", http.MethodDelete, "/api/v1/groups/" + groupID, ana},
	} {
		if rec := out.by.send(out.method, out.target, ""); rec.Code != http.StatusNoContent {
			t.Errorf("%s: code = %d, want 204 (body %s)", out.what, rec.Code, rec.Body)
		}
	}
}
