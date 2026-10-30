package httpapi_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
	"github.com/Aegisystem/Quitsie/api/internal/mail"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// world is the API over a real database, with real sessions: the handler is
// wrapped in auth.Service.Authenticate, as the router will do.
type world struct {
	t    *testing.T
	db   *sql.DB
	auth *auth.Service
	mail *outbox
	h    http.Handler
}

func newWorld(t *testing.T) *world {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "quitsie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	w := &world{t: t, db: db, mail: &outbox{}}
	w.auth, err = auth.New(auth.Config{DB: db, Mail: w.mail, DefaultCurrency: "COP"})
	if err != nil {
		t.Fatal(err)
	}
	server := httpapi.NewServer(db, "test").WithDeps(httpapi.Deps{DB: db, InviteSecret: []byte("a test secret of at least 32 bytes")})
	w.h = w.auth.Authenticate(httpapi.NewHandler(server, builtPWA))
	return w
}

// person is a signed-in User.
type person struct {
	w     *world
	id    string
	name  string
	email string
	token string
}

// signIn signs a new User in with a login code, the way the PWA does, and
// gives them a name.
func (w *world) signIn(name string) *person {
	w.t.Helper()
	ctx := context.Background()
	email := strings.ToLower(name) + "@example.com"
	// One IP per person, so the limit of codes per IP never trips.
	if err := w.auth.RequestCode(ctx, email, email); err != nil {
		w.t.Fatal(err)
	}
	signIn, err := w.auth.VerifyCode(ctx, email, w.mail.lastCode(w.t, email), "test")
	if err != nil {
		w.t.Fatal(err)
	}
	// The profile endpoint that sets the name belongs to another area.
	if _, err := w.db.Exec(`UPDATE users SET name = ? WHERE id = ?`, name, signIn.User.ID); err != nil {
		w.t.Fatal(err)
	}
	return &person{w: w, id: signIn.User.ID, name: name, email: email, token: signIn.Token}
}

// send makes a request with p's session cookie.
func (p *person) send(method, target, body string) *httptest.ResponseRecorder {
	p.w.t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.token != "" {
		req.AddCookie(auth.SessionCookie(p.token))
	}
	rec := httptest.NewRecorder()
	p.w.h.ServeHTTP(rec, req)
	return rec
}

// pull is everything p's replica would hold after a sync from scratch.
func (p *person) pull() store.Changes {
	p.w.t.Helper()
	changes, err := store.ChangesSince(context.Background(), p.w.db, p.id, 0)
	if err != nil {
		p.w.t.Fatal(err)
	}
	return changes
}

// decode checks the status of rec and decodes its JSON body.
func decode[T any](t *testing.T, rec *httptest.ResponseRecorder, status int) T {
	t.Helper()
	var body T
	if rec.Code != status {
		t.Fatalf("code = %d, want %d (body %q)", rec.Code, status, rec.Body.String())
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

// activityKinds lists the kinds of the Activities of ledgerID, oldest first.
func activityKinds(c store.Changes, ledgerID string) []string {
	var kinds []string
	for _, a := range c.Activities {
		if a.LedgerID == ledgerID {
			kinds = append(kinds, a.Kind)
		}
	}
	return kinds
}

// membersOf lists the Members of ledgerID in c.
func membersOf(c store.Changes, ledgerID string) []store.Member {
	var members []store.Member
	for _, m := range c.Members {
		if m.LedgerID == ledgerID {
			members = append(members, m)
		}
	}
	return members
}

// outbox is a mail.Sender that keeps the emails instead of sending them.
type outbox struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (o *outbox) Send(_ context.Context, m mail.Message) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sent = append(o.sent, m)
	return nil
}

var sixDigits = regexp.MustCompile(`\b\d{6}\b`)

// lastCode returns the login code in the last email sent to `to`.
func (o *outbox) lastCode(t *testing.T, to string) string {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()
	for i := len(o.sent) - 1; i >= 0; i-- {
		if o.sent[i].To == to {
			if code := sixDigits.FindString(o.sent[i].Text); code != "" {
				return code
			}
		}
	}
	t.Fatalf("no login code sent to %s", to)
	return ""
}

func TestCreateGroupMakesTheUserItsFirstMember(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")

	rec := ana.send(http.MethodPost, "/api/v1/groups", `{"name":"Cartagena","currency":"COP"}`)

	group := decode[httpapi.Group](t, rec, http.StatusCreated)
	if group.Name != "Cartagena" || group.Currency != "COP" || !group.SimplifyDebts {
		t.Errorf("group = %+v, want Cartagena in COP with Debt simplification on", group)
	}
	changes := ana.pull()
	members := membersOf(changes, group.Id.String())
	if len(members) != 1 || members[0].UserID == nil || *members[0].UserID != ana.id || members[0].Name != "Ana" || members[0].Status != "active" {
		t.Errorf("members = %+v, want only Ana, active and linked", members)
	}
	if kinds := activityKinds(changes, group.Id.String()); len(kinds) != 1 || kinds[0] != "group_created" {
		t.Errorf("activities = %v, want [group_created]", kinds)
	}
}

// anonymous is someone without a session.
func (w *world) anonymous() *person {
	return &person{w: w}
}

// createGroup creates a Group in COP and returns its id.
func (p *person) createGroup(body string) string {
	p.w.t.Helper()
	return decode[httpapi.Group](p.w.t, p.send(http.MethodPost, "/api/v1/groups", body), http.StatusCreated).Id.String()
}

func TestCreateGroupAddsThePendingMembersAlongWithTheUser(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")

	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","simplifyDebts":false,"pendingMembers":[{"name":"Beto","email":"beto@example.com"},{"name":"Carla"}]}`)

	changes := ana.pull()
	if len(changes.Ledgers) != 1 || changes.Ledgers[0].SimplifyDebts {
		t.Errorf("ledgers = %+v, want the Group with Debt simplification off", changes.Ledgers)
	}
	var pending []string
	for _, m := range membersOf(changes, groupID) {
		if m.UserID == nil {
			email := ""
			if m.Email != nil {
				email = *m.Email
			}
			pending = append(pending, m.Name+" "+email)
		}
	}
	if strings.Join(pending, ",") != "Beto beto@example.com,Carla " {
		t.Errorf("pending members = %q, want Beto with his email and Carla", pending)
	}
}

func TestCreateGroupNeedsASession(t *testing.T) {
	w := newWorld(t)

	rec := w.anonymous().send(http.MethodPost, "/api/v1/groups", `{"name":"Casa","currency":"COP"}`)

	decodeProblem(t, rec, http.StatusUnauthorized, httpapi.ProblemCodeUnauthenticated)
}

func TestCreateGroupRejectsWhatBreaksTheContract(t *testing.T) {
	tooMany := strings.Repeat(`{"name":"X"},`, 51)
	tests := []struct {
		name string
		body string
	}{
		{"blank name", `{"name":"  ","currency":"COP"}`},
		{"name over 80 characters", `{"name":"` + strings.Repeat("a", 81) + `","currency":"COP"}`},
		{"unknown currency", `{"name":"Casa","currency":"ABC"}`},
		{"lower-case currency", `{"name":"Casa","currency":"cop"}`},
		{"pending member without a name", `{"name":"Casa","currency":"COP","pendingMembers":[{"name":""}]}`},
		{"more than 50 pending members", `{"name":"Casa","currency":"COP","pendingMembers":[` + strings.TrimSuffix(tooMany, ",") + `]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := newWorld(t)
			ana := w.signIn("Ana")

			rec := ana.send(http.MethodPost, "/api/v1/groups", tt.body)

			decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
			if got := ana.pull().Ledgers; len(got) != 0 {
				t.Errorf("a Group was created anyway: %+v", got)
			}
		})
	}
}

func TestUpdateGroupRenamesItAndTurnsDebtSimplificationOff(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	rec := ana.send(http.MethodPatch, "/api/v1/groups/"+groupID, `{"name":"Casa 2026","simplifyDebts":false}`)

	group := decode[httpapi.Group](t, rec, http.StatusOK)
	if group.Name != "Casa 2026" || group.SimplifyDebts {
		t.Errorf("group = %+v, want renamed with Debt simplification off", group)
	}
	want := "group_created,group_renamed,debt_simplification_changed"
	if kinds := strings.Join(activityKinds(ana.pull(), groupID), ","); kinds != want {
		t.Errorf("activities = %s, want %s", kinds, want)
	}
}

func TestUpdateGroupRecordsOnlyWhatChanged(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	rec := ana.send(http.MethodPatch, "/api/v1/groups/"+groupID, `{"name":"Casa","simplifyDebts":false}`)

	decode[httpapi.Group](t, rec, http.StatusOK)
	want := "group_created,debt_simplification_changed"
	if kinds := strings.Join(activityKinds(ana.pull(), groupID), ","); kinds != want {
		t.Errorf("activities = %s, want %s", kinds, want)
	}
}

func TestUpdateGroupNeedsSomethingToChange(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	rec := ana.send(http.MethodPatch, "/api/v1/groups/"+groupID, `{}`)

	decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
}

func TestAGroupIsNotFoundForWhoeverIsNotInIt(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	for name, target := range map[string]string{
		"someone else's Group":        "/api/v1/groups/" + groupID,
		"a Group that does not exist": "/api/v1/groups/" + someID,
	} {
		t.Run(name, func(t *testing.T) {
			rec := beto.send(http.MethodPatch, target, `{"name":"Mía"}`)

			decodeProblem(t, rec, http.StatusNotFound, httpapi.ProblemCodeNotFound)
		})
	}
}

// memberID is the id of the Member called name in ledgerID, as p sees it.
func (p *person) memberID(ledgerID, name string) string {
	p.w.t.Helper()
	for _, m := range membersOf(p.pull(), ledgerID) {
		if m.Name == name {
			return m.ID
		}
	}
	p.w.t.Fatalf("no Member called %s in %s", name, ledgerID)
	return ""
}

// expense stores an Expense that payer paid in full and owed splits, straight
// in the store: pushing records is an endpoint of another area. It returns
// the Expense's id.
func (p *person) expense(ledgerID, currency, payer string, owed map[string]int64) string {
	p.w.t.Helper()
	ctx := context.Background()
	id := uuid.Must(uuid.NewV7()).String()
	e := store.ExpenseWithShares{Expense: store.Expense{
		ID:          id,
		LedgerID:    ledgerID,
		Description: "Mercado",
		Date:        time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC),
		Currency:    currency,
		SplitMethod: "exact",
		CreatedBy:   p.id,
		CreatedAt:   time.Now().UTC(),
	}}
	for member, amount := range owed {
		e.Total += amount
		e.OwedShares = append(e.OwedShares, store.OwedShare{MemberID: member, Amount: amount})
	}
	e.PaidShares = []store.PaidShare{{MemberID: payer, Amount: e.Total}}
	_, err := store.Write(ctx, p.w.db, func(q *store.Queries, version int64) error {
		e.Version = version
		_, _, err := q.CreateExpense(ctx, e)
		return err
	})
	if err != nil {
		p.w.t.Fatal(err)
	}
	return id
}

// deleteExpense deletes an Expense straight in the store; see expense.
func (w *world) deleteExpense(id string) {
	w.t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := store.Write(ctx, w.db, func(q *store.Queries, version int64) error {
		_, err := q.DeleteExpense(ctx, store.DeleteExpenseParams{ID: id, DeletedAt: &now, Version: version})
		return err
	})
	if err != nil {
		w.t.Fatal(err)
	}
}

// totals lists "id currency total" for every Expense in c.
func totals(c store.Changes) map[string]string {
	out := map[string]string{}
	for _, e := range c.Expenses {
		out[e.ID] = fmt.Sprintf("%s %d", e.Currency, e.Total)
	}
	return out
}

func TestChangingTheGroupCurrencyRelabelsEveryRecordKeepingItsNumbers(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Viaje","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	anaID, betoID := ana.memberID(groupID, "Ana"), ana.memberID(groupID, "Beto")
	live := ana.expense(groupID, "COP", anaID, map[string]int64{anaID: 4500, betoID: 4500})
	deleted := ana.expense(groupID, "COP", anaID, map[string]int64{betoID: 1200})
	w.deleteExpense(deleted)

	rec := ana.send(http.MethodPatch, "/api/v1/groups/"+groupID, `{"currency":"CLP"}`)

	group := decode[httpapi.Group](t, rec, http.StatusOK)
	if group.Currency != "CLP" {
		t.Errorf("currency = %s, want CLP", group.Currency)
	}
	changes := ana.pull()
	// 90.00 COP is 90 CLP: the same number in a currency without decimals.
	want := map[string]string{live: "CLP 90", deleted: "CLP 12"}
	if got := totals(changes); !maps.Equal(got, want) {
		t.Errorf("Expenses = %v, want %v", got, want)
	}
	for _, e := range changes.Expenses {
		for _, s := range e.OwedShares {
			if e.ID == live && s.Amount != 45 {
				t.Errorf("OwedShare of %s in %s = %d, want 45", s.MemberID, e.ID, s.Amount)
			}
		}
	}
	if kinds := activityKinds(changes, groupID); kinds[len(kinds)-1] != "group_currency_changed" {
		t.Errorf("activities = %v, want the last one to be group_currency_changed", kinds)
	}
}

func TestTheGroupCurrencyCannotChangeIfSomeAmountWouldLoseDecimals(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Viaje","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	anaID, betoID := ana.memberID(groupID, "Ana"), ana.memberID(groupID, "Beto")
	ana.expense(groupID, "COP", anaID, map[string]int64{betoID: 9000})
	// 10.50 COP cannot be written in CLP, and a deleted Expense can come back.
	cents := ana.expense(groupID, "COP", anaID, map[string]int64{betoID: 1050})
	w.deleteExpense(cents)
	before := totals(ana.pull())

	rec := ana.send(http.MethodPatch, "/api/v1/groups/"+groupID, `{"name":"Viaje 2026","currency":"CLP"}`)

	p := decode[httpapi.CurrencyPrecisionProblem](t, rec, http.StatusConflict)
	if p.Code != httpapi.ProblemCodeCurrencyPrecisionLoss || len(p.ExpenseIds) != 1 || p.ExpenseIds[0].String() != cents || len(p.SettlementIds) != 0 {
		t.Errorf("problem = %+v, want currency_precision_loss naming only %s", p, cents)
	}
	changes := ana.pull()
	if got := totals(changes); !maps.Equal(got, before) {
		t.Errorf("Expenses = %v, want them untouched: %v", got, before)
	}
	if changes.Ledgers[0].Name != "Viaje" {
		t.Errorf("the Group was renamed anyway: %+v", changes.Ledgers[0])
	}
}

func TestDeleteGroupWhenEveryBalanceIsZero(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	// Ana paid only her own share: nobody owes anything.
	ana.expense(groupID, "COP", ana.memberID(groupID, "Ana"), map[string]int64{ana.memberID(groupID, "Ana"): 5000})

	rec := ana.send(http.MethodDelete, "/api/v1/groups/"+groupID, "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204 (body %q)", rec.Code, rec.Body.String())
	}
	changes := ana.pull()
	if len(changes.Ledgers) != 1 || changes.Ledgers[0].DeletedAt == nil {
		t.Errorf("ledgers = %+v, want the Group as a deletion", changes.Ledgers)
	}
	if kinds := strings.Join(activityKinds(changes, groupID), ","); kinds != "group_created,group_deleted" {
		t.Errorf("activities = %s, want group_created,group_deleted", kinds)
	}
	again := ana.send(http.MethodPatch, "/api/v1/groups/"+groupID, `{"name":"Otra"}`)
	decodeProblem(t, again, http.StatusNotFound, httpapi.ProblemCodeNotFound)
}

func TestAGroupWithSomeBalanceCannotBeDeleted(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	ana.expense(groupID, "COP", ana.memberID(groupID, "Ana"), map[string]int64{ana.memberID(groupID, "Beto"): 5000})

	rec := ana.send(http.MethodDelete, "/api/v1/groups/"+groupID, "")

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeNonzeroBalance)
	if ledgers := ana.pull().Ledgers; len(ledgers) != 1 || ledgers[0].DeletedAt != nil {
		t.Errorf("ledgers = %+v, want the Group alive", ledgers)
	}
}

func TestAddPendingMember(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	rec := ana.send(http.MethodPost, "/api/v1/groups/"+groupID+"/members", `{"name":" Carla ","email":"carla@example.com"}`)

	m := decode[httpapi.Member](t, rec, http.StatusCreated)
	if m.Name != "Carla" || m.Email == nil || *m.Email != "carla@example.com" || m.UserId != nil || m.Status != httpapi.Active || m.LedgerId.String() != groupID {
		t.Errorf("member = %+v, want Carla pending in the Group with her email", m)
	}
	changes := ana.pull()
	if len(membersOf(changes, groupID)) != 2 {
		t.Errorf("members = %+v, want Ana and Carla", membersOf(changes, groupID))
	}
	if kinds := strings.Join(activityKinds(changes, groupID), ","); kinds != "group_created,member_added" {
		t.Errorf("activities = %s, want group_created,member_added", kinds)
	}
}

func TestAddPendingMemberNeedsAName(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	rec := ana.send(http.MethodPost, "/api/v1/groups/"+groupID+"/members", `{"name":""}`)

	decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
}

// member is the Member memberID as p sees it.
func (p *person) member(ledgerID, memberID string) store.Member {
	p.w.t.Helper()
	for _, m := range membersOf(p.pull(), ledgerID) {
		if m.ID == memberID {
			return m
		}
	}
	p.w.t.Fatalf("no Member %s in %s", memberID, ledgerID)
	return store.Member{}
}

func TestRemoveAPendingMemberWithBalanceZero(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	betoID := ana.memberID(groupID, "Beto")

	rec := ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+betoID, "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204 (body %q)", rec.Code, rec.Body.String())
	}
	if m := ana.member(groupID, betoID); m.Status != "former" {
		t.Errorf("Beto = %+v, want a former member", m)
	}
	if kinds := strings.Join(activityKinds(ana.pull(), groupID), ","); kinds != "group_created,member_removed" {
		t.Errorf("activities = %s, want group_created,member_removed", kinds)
	}
}

func TestAMemberWithSomeBalanceCannotBeRemoved(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	betoID := ana.memberID(groupID, "Beto")
	ana.expense(groupID, "COP", ana.memberID(groupID, "Ana"), map[string]int64{betoID: 5000})

	rec := ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+betoID, "")

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeNonzeroBalance)
	if m := ana.member(groupID, betoID); m.Status != "active" {
		t.Errorf("Beto = %+v, want still active", m)
	}
}

func TestRemoveMemberOnlyFindsActiveMembersOfThatGroup(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	otherID := ana.createGroup(`{"name":"Oficina","currency":"COP","pendingMembers":[{"name":"Carla"}]}`)
	betoID := ana.memberID(groupID, "Beto")
	ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+betoID, "")

	for name, memberID := range map[string]string{
		"a former member":             betoID,
		"a Member of another Group":   ana.memberID(otherID, "Carla"),
		"a Member that never existed": someID,
	} {
		t.Run(name, func(t *testing.T) {
			rec := ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+memberID, "")

			decodeProblem(t, rec, http.StatusNotFound, httpapi.ProblemCodeNotFound)
		})
	}
}

func TestLeaveGroupWithBalanceZero(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)

	rec := ana.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204 (body %q)", rec.Code, rec.Body.String())
	}
	if dropped := ana.pull().DroppedLedgerIDs; len(dropped) != 1 || dropped[0] != groupID {
		t.Errorf("dropped ledgers = %v, want the Group", dropped)
	}
	again := ana.send(http.MethodPatch, "/api/v1/groups/"+groupID, `{"name":"Otra"}`)
	decodeProblem(t, again, http.StatusNotFound, httpapi.ProblemCodeNotFound)
}

func TestNobodyLeavesAGroupOwingOrBeingOwed(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"}]}`)
	ana.expense(groupID, "COP", ana.memberID(groupID, "Ana"), map[string]int64{ana.memberID(groupID, "Beto"): 5000})

	rec := ana.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", "")

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeNonzeroBalance)
	if dropped := ana.pull().DroppedLedgerIDs; len(dropped) != 0 {
		t.Errorf("dropped ledgers = %v, want none", dropped)
	}
}

// groupLink is the token of the Group's current Invite link.
func (p *person) groupLink(groupID string) string {
	p.w.t.Helper()
	return decode[httpapi.InviteLink](p.w.t, p.send(http.MethodGet, "/api/v1/groups/"+groupID+"/invite-link", ""), http.StatusOK).Token
}

func TestTheGroupInviteLinkCanBeShownAgainUntilRegenerated(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	first := ana.groupLink(groupID)
	if again := ana.groupLink(groupID); again != first {
		t.Fatalf("the link changed from %q to %q without regenerating it", first, again)
	}

	rec := ana.send(http.MethodPost, "/api/v1/groups/"+groupID+"/invite-link", "")

	regenerated := decode[httpapi.InviteLink](t, rec, http.StatusOK).Token
	if regenerated == first || ana.groupLink(groupID) != regenerated {
		t.Errorf("regenerated %q from %q, and the Group now shows %q", regenerated, first, ana.groupLink(groupID))
	}
	decodeProblem(t, w.anonymous().send(http.MethodGet, "/api/v1/invites/"+first, ""), http.StatusNotFound, httpapi.ProblemCodeInviteLinkInvalid)
	decode[httpapi.Invite](t, w.anonymous().send(http.MethodGet, "/api/v1/invites/"+regenerated, ""), http.StatusOK)
	if kinds := strings.Join(activityKinds(ana.pull(), groupID), ","); kinds != "group_created,invite_link_regenerated" {
		t.Errorf("activities = %s, want group_created,invite_link_regenerated", kinds)
	}
}

func TestOnlyMembersSeeTheGroupInviteLink(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	decodeProblem(t, beto.send(http.MethodGet, "/api/v1/groups/"+groupID+"/invite-link", ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
	decodeProblem(t, beto.send(http.MethodPost, "/api/v1/groups/"+groupID+"/invite-link", ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
}

func TestAnInviteShowsTheGroupAndItsPendingMembersWithoutASession(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto"},{"name":"Carla"},{"name":"Dani"}]}`)
	ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+ana.memberID(groupID, "Dani"), "")

	rec := w.anonymous().send(http.MethodGet, "/api/v1/invites/"+ana.groupLink(groupID), "")

	invite := decode[httpapi.Invite](t, rec, http.StatusOK)
	if invite.Kind != httpapi.InviteKindGroup || invite.LedgerId.String() != groupID || invite.GroupName == nil || *invite.GroupName != "Casa" {
		t.Errorf("invite = %+v, want the Group Casa", invite)
	}
	var names []string
	for _, m := range invite.PendingMembers {
		names = append(names, m.Name)
	}
	if strings.Join(names, ",") != "Beto,Carla" {
		t.Errorf("pending members = %v, want Beto and Carla: Dani was removed and Ana is linked", names)
	}
	if invite.SuggestedMemberId != nil || invite.JoinedMemberId != nil {
		t.Errorf("invite = %+v, want no suggestion and no Member without a session", invite)
	}
}

func TestAnInviteSuggestsThePendingMemberWithTheUsersEmail(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Carla"},{"name":"Betico","email":"Beto@Example.com"}]}`)

	rec := beto.send(http.MethodGet, "/api/v1/invites/"+ana.groupLink(groupID), "")

	invite := decode[httpapi.Invite](t, rec, http.StatusOK)
	if invite.SuggestedMemberId == nil || invite.SuggestedMemberId.String() != ana.memberID(groupID, "Betico") {
		t.Errorf("suggested member = %v, want Betico, whose email is Beto's", invite.SuggestedMemberId)
	}
	if invite.JoinedMemberId != nil {
		t.Errorf("joined member = %v, want none: Beto is not in the Group", invite.JoinedMemberId)
	}
	// A suggestion never links anyone.
	if m := ana.member(groupID, ana.memberID(groupID, "Betico")); m.UserID != nil {
		t.Errorf("Betico = %+v, want still pending", m)
	}
}

func TestAnInviteSaysWhichMemberTheUserAlreadyHolds(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	rec := ana.send(http.MethodGet, "/api/v1/invites/"+ana.groupLink(groupID), "")

	invite := decode[httpapi.Invite](t, rec, http.StatusOK)
	if invite.JoinedMemberId == nil || invite.JoinedMemberId.String() != ana.memberID(groupID, "Ana") {
		t.Errorf("joined member = %v, want Ana's", invite.JoinedMemberId)
	}
}

func TestInvalidInviteLinks(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	deletedID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	deletedLink := ana.groupLink(deletedID)
	ana.send(http.MethodDelete, "/api/v1/groups/"+deletedID, "")

	for name, token := range map[string]string{
		"made up":            "k3J9xQ2mV7pL8wR4tY6u",
		"of a deleted Group": deletedLink,
	} {
		t.Run(name, func(t *testing.T) {
			rec := ana.send(http.MethodGet, "/api/v1/invites/"+token, "")

			decodeProblem(t, rec, http.StatusNotFound, httpapi.ProblemCodeInviteLinkInvalid)
		})
	}
}

// join makes p join through the Invite link token.
func (p *person) join(token string) httpapi.JoinedLedger {
	p.w.t.Helper()
	return decode[httpapi.JoinedLedger](p.w.t, p.send(http.MethodPost, "/api/v1/invites/"+token+"/join", ""), http.StatusCreated)
}

// claim makes p claim memberID through the Invite link token.
func (p *person) claim(token, memberID string) *httptest.ResponseRecorder {
	p.w.t.Helper()
	return p.send(http.MethodPost, "/api/v1/invites/"+token+"/claim", `{"memberId":"`+memberID+`"}`)
}

func TestJoinGroupAddsTheUserAsANewMember(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	joined := beto.join(ana.groupLink(groupID))

	if joined.Kind != httpapi.JoinedLedgerKindGroup || joined.LedgerId.String() != groupID {
		t.Errorf("joined = %+v, want the Group", joined)
	}
	m := beto.member(groupID, joined.MemberId.String())
	if m.Name != "Beto" || m.UserID == nil || *m.UserID != beto.id || m.Status != "active" {
		t.Errorf("Beto's member = %+v, want active, linked and named after him", m)
	}
	if kinds := strings.Join(activityKinds(beto.pull(), groupID), ","); kinds != "group_created,member_joined" {
		t.Errorf("activities = %s, want group_created,member_joined", kinds)
	}
}

func TestJoiningAGroupTwiceIsAConflict(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	rec := ana.send(http.MethodPost, "/api/v1/invites/"+ana.groupLink(groupID)+"/join", "")

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeAlreadyMember)
}

func TestComingBackToAGroupRecoversTheFormerMember(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	first := beto.join(ana.groupLink(groupID))
	beto.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", "")

	again := beto.join(ana.groupLink(groupID))

	if again.MemberId != first.MemberId {
		t.Errorf("Beto came back as %s, want his former member %s", again.MemberId, first.MemberId)
	}
	if members := membersOf(ana.pull(), groupID); len(members) != 2 {
		t.Errorf("members = %+v, want only Ana and Beto", members)
	}
	if m := beto.member(groupID, first.MemberId.String()); m.Status != "active" {
		t.Errorf("Beto's member = %+v, want active again", m)
	}
}

func TestJoinGroupNeedsASessionAndALiveLink(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	revoked := ana.groupLink(groupID)
	ana.send(http.MethodPost, "/api/v1/groups/"+groupID+"/invite-link", "")

	decodeProblem(t, w.anonymous().send(http.MethodPost, "/api/v1/invites/"+ana.groupLink(groupID)+"/join", ""), http.StatusUnauthorized, httpapi.ProblemCodeUnauthenticated)
	decodeProblem(t, beto.send(http.MethodPost, "/api/v1/invites/"+revoked+"/join", ""), http.StatusNotFound, httpapi.ProblemCodeInviteLinkInvalid)
}

func TestClaimAPendingMemberInheritsItsHistory(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Betico"}]}`)
	beticoID := ana.memberID(groupID, "Betico")
	spent := ana.expense(groupID, "COP", ana.memberID(groupID, "Ana"), map[string]int64{beticoID: 5000})

	rec := beto.claim(ana.groupLink(groupID), beticoID)

	joined := decode[httpapi.JoinedLedger](t, rec, http.StatusOK)
	if joined.MemberId.String() != beticoID || joined.LedgerId.String() != groupID {
		t.Errorf("joined = %+v, want Betico in the Group", joined)
	}
	changes := beto.pull()
	if m := beto.member(groupID, beticoID); m.UserID == nil || *m.UserID != beto.id {
		t.Errorf("Betico = %+v, want linked to Beto", m)
	}
	if _, ok := totals(changes)[spent]; !ok {
		t.Errorf("Beto's replica lacks the Expense Betico was in: %v", totals(changes))
	}
	if kinds := strings.Join(activityKinds(changes, groupID), ","); kinds != "group_created,member_claimed" {
		t.Errorf("activities = %s, want group_created,member_claimed", kinds)
	}
}

func TestClaimRefusals(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	carla := w.signIn("Carla")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Betico"},{"name":"Dani"},{"name":"Eva"}]}`)
	otherID := ana.createGroup(`{"name":"Oficina","currency":"COP","pendingMembers":[{"name":"Fede"}]}`)
	link := ana.groupLink(groupID)
	ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+ana.memberID(groupID, "Eva"), "")
	carla.join(link)
	carla.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", "")

	tests := []struct {
		name     string
		who      *person
		memberID string
		status   int
		code     httpapi.ProblemCode
	}{
		{"a linked Member", beto, ana.memberID(groupID, "Ana"), http.StatusConflict, httpapi.ProblemCodeMemberNotPending},
		{"while holding a Member of the Group", ana, ana.memberID(groupID, "Betico"), http.StatusConflict, httpapi.ProblemCodeAlreadyMember},
		{"after leaving the Group", carla, ana.memberID(groupID, "Dani"), http.StatusConflict, httpapi.ProblemCodeAlreadyMember},
		{"a removed pending Member", beto, ana.memberID(groupID, "Eva"), http.StatusNotFound, httpapi.ProblemCodeNotFound},
		{"a Member of another Group", beto, ana.memberID(otherID, "Fede"), http.StatusNotFound, httpapi.ProblemCodeNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.who.claim(link, tt.memberID)

			decodeProblem(t, rec, tt.status, tt.code)
		})
	}
}

func TestUndoAClaimMadeByMistake(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Carla"}]}`)
	carlaID := ana.memberID(groupID, "Carla")
	decode[httpapi.JoinedLedger](t, beto.claim(ana.groupLink(groupID), carlaID), http.StatusOK)

	rec := ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+carlaID+"/claim", "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204 (body %q)", rec.Code, rec.Body.String())
	}
	if m := ana.member(groupID, carlaID); m.UserID != nil || m.Name != "Carla" || m.Status != "active" {
		t.Errorf("Carla = %+v, want pending again with her name", m)
	}
	// Beto no longer sees the Group.
	decodeProblem(t, beto.send(http.MethodGet, "/api/v1/groups/"+groupID+"/invite-link", ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
	want := "group_created,member_claimed,claim_undone"
	if kinds := strings.Join(activityKinds(ana.pull(), groupID), ","); kinds != want {
		t.Errorf("activities = %s, want %s", kinds, want)
	}
}

func TestOnlyALinkedMemberHasAClaimToUndo(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Carla"}]}`)

	decodeProblem(t, ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+ana.memberID(groupID, "Carla")+"/claim", ""), http.StatusConflict, httpapi.ProblemCodeMemberNotClaimed)
	decodeProblem(t, ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+someID+"/claim", ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
}

func TestARemovedLinkedMemberLosesAccessToTheGroup(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	joined := beto.join(ana.groupLink(groupID))

	rec := ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+joined.MemberId.String(), "")

	if rec.Code != http.StatusNoContent {
		t.Fatalf("code = %d, want 204 (body %q)", rec.Code, rec.Body.String())
	}
	if dropped := beto.pull().DroppedLedgerIDs; len(dropped) != 1 || dropped[0] != groupID {
		t.Errorf("Beto's dropped ledgers = %v, want the Group", dropped)
	}
	decodeProblem(t, beto.send(http.MethodGet, "/api/v1/groups/"+groupID+"/invite-link", ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
}

// createFriendship starts a Friendship and returns its id.
func (p *person) createFriendship(body string) string {
	p.w.t.Helper()
	return decode[httpapi.Friendship](p.w.t, p.send(http.MethodPost, "/api/v1/friendships", body), http.StatusCreated).Id.String()
}

func TestCreateFriendshipWithAPendingFriend(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	// Beto has an account with that email, and even so it never links him.
	w.signIn("Beto")

	friendshipID := ana.createFriendship(`{"name":" Beto ","email":"beto@example.com"}`)

	changes := ana.pull()
	if len(changes.Ledgers) != 1 || changes.Ledgers[0].Kind != "friendship" || changes.Ledgers[0].Currency != nil {
		t.Errorf("ledgers = %+v, want one Friendship without a currency", changes.Ledgers)
	}
	var linked, pending []string
	for _, m := range membersOf(changes, friendshipID) {
		if m.UserID != nil {
			linked = append(linked, m.Name)
		} else if m.Email != nil {
			pending = append(pending, m.Name+" "+*m.Email)
		}
	}
	if strings.Join(linked, ",") != "Ana" || strings.Join(pending, ",") != "Beto beto@example.com" {
		t.Errorf("linked = %v, pending = %v; want Ana linked and Beto pending with his email", linked, pending)
	}
	if kinds := strings.Join(activityKinds(changes, friendshipID), ","); kinds != "friendship_created" {
		t.Errorf("activities = %s, want friendship_created", kinds)
	}
}

func TestCreateFriendshipWithSomeoneFromAGroup(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	beto.join(ana.groupLink(ana.createGroup(`{"name":"Casa","currency":"COP"}`)))

	friendshipID := ana.createFriendship(`{"userId":"` + beto.id + `"}`)

	members := membersOf(beto.pull(), friendshipID)
	if len(members) != 2 {
		t.Fatalf("members = %+v, want Ana and Beto", members)
	}
	for _, m := range members {
		if m.UserID == nil || m.Status != "active" {
			t.Errorf("member = %+v, want linked and active", m)
		}
	}
}

func TestAFriendshipWithAUserNeedsAGroupInCommon(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	carla := w.signIn("Carla")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	carla.join(ana.groupLink(groupID))
	carla.send(http.MethodPost, "/api/v1/groups/"+groupID+"/leave", "")

	for name, userID := range map[string]string{
		"no Group ever":              beto.id,
		"a Group the other one left": carla.id,
		"a User that does not exist": someID,
	} {
		t.Run(name, func(t *testing.T) {
			rec := ana.send(http.MethodPost, "/api/v1/friendships", `{"userId":"`+userID+`"}`)

			decodeProblem(t, rec, http.StatusNotFound, httpapi.ProblemCodeNotFound)
		})
	}
}

func TestThereIsOnlyOneFriendshipBetweenTwoUsers(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	beto.join(ana.groupLink(ana.createGroup(`{"name":"Casa","currency":"COP"}`)))
	ana.createFriendship(`{"userId":"` + beto.id + `"}`)

	decodeProblem(t, ana.send(http.MethodPost, "/api/v1/friendships", `{"userId":"`+beto.id+`"}`), http.StatusConflict, httpapi.ProblemCodeFriendshipExists)
	decodeProblem(t, beto.send(http.MethodPost, "/api/v1/friendships", `{"userId":"`+ana.id+`"}`), http.StatusConflict, httpapi.ProblemCodeFriendshipExists)
}

func TestCreateFriendshipRejectsWhatBreaksTheContract(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")

	for name, body := range map[string]string{
		"nobody":                 `{}`,
		"only an email":          `{"email":"beto@example.com"}`,
		"blank name":             `{"name":"   "}`,
		"both a User and a name": `{"userId":"` + beto.id + `","name":"Beto"}`,
		"a User and an email":    `{"userId":"` + beto.id + `","email":"beto@example.com"}`,
		"the User themselves":    `{"userId":"` + ana.id + `"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := ana.send(http.MethodPost, "/api/v1/friendships", body)

			decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
		})
	}
}

func TestCreateFriendshipNeedsASession(t *testing.T) {
	w := newWorld(t)

	rec := w.anonymous().send(http.MethodPost, "/api/v1/friendships", `{"name":"Beto"}`)

	decodeProblem(t, rec, http.StatusUnauthorized, httpapi.ProblemCodeUnauthenticated)
}

// friendLink is the token of the pending Friend's current Invite link.
func (p *person) friendLink(friendshipID string) string {
	p.w.t.Helper()
	return decode[httpapi.InviteLink](p.w.t, p.send(http.MethodGet, "/api/v1/friendships/"+friendshipID+"/invite-link", ""), http.StatusOK).Token
}

func TestThePendingFriendLinkCanBeShownAgainUntilRegenerated(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	friendshipID := ana.createFriendship(`{"name":"Beto"}`)
	first := ana.friendLink(friendshipID)
	if again := ana.friendLink(friendshipID); again != first {
		t.Fatalf("the link changed from %q to %q without regenerating it", first, again)
	}

	rec := ana.send(http.MethodPost, "/api/v1/friendships/"+friendshipID+"/invite-link", "")

	regenerated := decode[httpapi.InviteLink](t, rec, http.StatusOK).Token
	if regenerated == first || ana.friendLink(friendshipID) != regenerated {
		t.Errorf("regenerated %q from %q, and the Friendship now shows %q", regenerated, first, ana.friendLink(friendshipID))
	}
	decodeProblem(t, w.anonymous().send(http.MethodGet, "/api/v1/invites/"+first, ""), http.StatusNotFound, httpapi.ProblemCodeInviteLinkInvalid)
	if kinds := strings.Join(activityKinds(ana.pull(), friendshipID), ","); kinds != "friendship_created,invite_link_regenerated" {
		t.Errorf("activities = %s, want friendship_created,invite_link_regenerated", kinds)
	}
}

func TestOnlyTheFriendshipsMembersSeeItsLink(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	friendshipID := ana.createFriendship(`{"name":"Carla"}`)
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)

	decodeProblem(t, beto.send(http.MethodGet, "/api/v1/friendships/"+friendshipID+"/invite-link", ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
	decodeProblem(t, beto.send(http.MethodPost, "/api/v1/friendships/"+friendshipID+"/invite-link", ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
	decodeProblem(t, ana.send(http.MethodGet, "/api/v1/friendships/"+groupID+"/invite-link", ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
	decodeProblem(t, ana.send(http.MethodGet, "/api/v1/groups/"+friendshipID+"/invite-link", ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
}

func TestAnInviteToAFriendshipShowsWhoInvitesAndThePendingFriend(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	friendshipID := ana.createFriendship(`{"name":"Betico","email":"beto@example.com"}`)
	beto := w.signIn("Beto")

	rec := beto.send(http.MethodGet, "/api/v1/invites/"+ana.friendLink(friendshipID), "")

	invite := decode[httpapi.Invite](t, rec, http.StatusOK)
	if invite.Kind != httpapi.InviteKindFriendship || invite.InvitedBy == nil || *invite.InvitedBy != "Ana" || invite.GroupName != nil {
		t.Errorf("invite = %+v, want a Friendship invited by Ana, without a Group name", invite)
	}
	friendID := ana.memberID(friendshipID, "Betico")
	if len(invite.PendingMembers) != 1 || invite.PendingMembers[0].Id.String() != friendID {
		t.Errorf("pending members = %+v, want only Betico", invite.PendingMembers)
	}
	if invite.SuggestedMemberId == nil || invite.SuggestedMemberId.String() != friendID {
		t.Errorf("suggested member = %v, want Betico, whose email is Beto's", invite.SuggestedMemberId)
	}
}

func TestClaimAPendingFriendLinksTheFriendship(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	friendshipID := ana.createFriendship(`{"name":"Betico"}`)
	friendID := ana.memberID(friendshipID, "Betico")
	link := ana.friendLink(friendshipID)
	spent := ana.expense(friendshipID, "USD", ana.memberID(friendshipID, "Ana"), map[string]int64{friendID: 700})

	rec := beto.claim(link, friendID)

	joined := decode[httpapi.JoinedLedger](t, rec, http.StatusOK)
	if joined.Kind != httpapi.JoinedLedgerKindFriendship || joined.MemberId.String() != friendID {
		t.Errorf("joined = %+v, want Betico's Friendship", joined)
	}
	changes := beto.pull()
	if _, ok := totals(changes)[spent]; !ok {
		t.Errorf("Beto's replica lacks the Expense Betico was in: %v", totals(changes))
	}
	if kinds := strings.Join(activityKinds(changes, friendshipID), ","); kinds != "friendship_created,member_claimed" {
		t.Errorf("activities = %s, want friendship_created,member_claimed", kinds)
	}
}

func TestAFriendLinkEndsWhenTheFriendIsLinked(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	carla := w.signIn("Carla")
	friendshipID := ana.createFriendship(`{"name":"Betico"}`)
	link := ana.friendLink(friendshipID)
	decode[httpapi.JoinedLedger](t, beto.claim(link, ana.memberID(friendshipID, "Betico")), http.StatusOK)

	target := "/api/v1/friendships/" + friendshipID + "/invite-link"
	decodeProblem(t, ana.send(http.MethodGet, target, ""), http.StatusConflict, httpapi.ProblemCodeMemberNotPending)
	decodeProblem(t, ana.send(http.MethodPost, target, ""), http.StatusConflict, httpapi.ProblemCodeMemberNotPending)
	decodeProblem(t, carla.send(http.MethodGet, "/api/v1/invites/"+link, ""), http.StatusNotFound, httpapi.ProblemCodeInviteLinkInvalid)
	// Whoever claimed it can reopen the link and is told they are already in.
	invite := decode[httpapi.Invite](t, beto.send(http.MethodGet, "/api/v1/invites/"+link, ""), http.StatusOK)
	if invite.JoinedMemberId == nil || len(invite.PendingMembers) != 0 {
		t.Errorf("invite = %+v, want Beto's Member and nobody pending", invite)
	}
}

func TestAFriendLinkOnlyAllowsClaiming(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	link := ana.friendLink(ana.createFriendship(`{"name":"Betico"}`))

	rec := beto.send(http.MethodPost, "/api/v1/invites/"+link+"/join", "")

	decodeProblem(t, rec, http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
}

func TestWhoAddedAPendingFriendCannotClaimIt(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	friendshipID := ana.createFriendship(`{"name":"Beto"}`)

	rec := ana.claim(ana.friendLink(friendshipID), ana.memberID(friendshipID, "Beto"))

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeAlreadyMember)
}

func TestClaimingAPendingFriendOfSomeoneAlreadyAFriendIsRefused(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	beto.join(ana.groupLink(ana.createGroup(`{"name":"Casa","currency":"COP"}`)))
	ana.createFriendship(`{"userId":"` + beto.id + `"}`)
	duplicate := ana.createFriendship(`{"name":"Betico"}`)

	rec := beto.claim(ana.friendLink(duplicate), ana.memberID(duplicate, "Betico"))

	decodeProblem(t, rec, http.StatusConflict, httpapi.ProblemCodeFriendshipExists)
	if m := ana.member(duplicate, ana.memberID(duplicate, "Betico")); m.UserID != nil {
		t.Errorf("Betico = %+v, want still pending", m)
	}
}

func TestAFriendLinkEndsWhenWhoAddedTheFriendDeletesTheirAccount(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	friendshipID := ana.createFriendship(`{"name":"Betico"}`)
	link := ana.friendLink(friendshipID)
	friendID := ana.memberID(friendshipID, "Betico")
	ctx := context.Background()
	_, err := store.Write(ctx, w.db, func(q *store.Queries, version int64) error {
		return q.UnlinkMembersOfUser(ctx, store.UnlinkMembersOfUserParams{UserID: ana.id, Version: version})
	})
	if err != nil {
		t.Fatal(err)
	}

	decodeProblem(t, beto.send(http.MethodGet, "/api/v1/invites/"+link, ""), http.StatusNotFound, httpapi.ProblemCodeInviteLinkInvalid)
	decodeProblem(t, beto.claim(link, friendID), http.StatusNotFound, httpapi.ProblemCodeInviteLinkInvalid)
}

func TestEditAPendingMember(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Betico"}]}`)
	betoID := ana.memberID(groupID, "Betico")

	rec := ana.send(http.MethodPut, "/api/v1/groups/"+groupID+"/members/"+betoID, `{"name":" Beto Ríos ","email":"beto@example.com"}`)

	m := decode[httpapi.Member](t, rec, http.StatusOK)
	if m.Name != "Beto Ríos" || m.Email == nil || *m.Email != "beto@example.com" || m.UserId != nil {
		t.Errorf("member = %+v, want pending with the new name and email", m)
	}
	if kinds := strings.Join(activityKinds(ana.pull(), groupID), ","); kinds != "group_created,member_updated" {
		t.Errorf("activities = %s, want group_created,member_updated", kinds)
	}
	// The new email is what the invite suggests from now on.
	invite := decode[httpapi.Invite](t, beto.send(http.MethodGet, "/api/v1/invites/"+ana.groupLink(groupID), ""), http.StatusOK)
	if invite.SuggestedMemberId == nil || invite.SuggestedMemberId.String() != betoID {
		t.Errorf("suggested member = %v, want %s", invite.SuggestedMemberId, betoID)
	}
}

func TestEditingAPendingMemberWithoutAnEmailRemovesIt(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Beto","email":"beto@example.com"}]}`)
	betoID := ana.memberID(groupID, "Beto")

	rec := ana.send(http.MethodPut, "/api/v1/groups/"+groupID+"/members/"+betoID, `{"name":"Beto"}`)

	if m := decode[httpapi.Member](t, rec, http.StatusOK); m.Email != nil {
		t.Errorf("email = %v, want none", *m.Email)
	}
}

func TestEditPendingMemberRefusals(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	beto := w.signIn("Beto")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP","pendingMembers":[{"name":"Carla"},{"name":"Dani"}]}`)
	otherID := ana.createGroup(`{"name":"Oficina","currency":"COP","pendingMembers":[{"name":"Eva"}]}`)
	carlaID := ana.memberID(groupID, "Carla")
	daniID := ana.memberID(groupID, "Dani")
	ana.send(http.MethodDelete, "/api/v1/groups/"+groupID+"/members/"+daniID, "")

	tests := []struct {
		name     string
		who      *person
		memberID string
		body     string
		status   int
		code     httpapi.ProblemCode
	}{
		{"a linked Member", ana, ana.memberID(groupID, "Ana"), `{"name":"Otra"}`, http.StatusConflict, httpapi.ProblemCodeMemberNotPending},
		{"a former member", ana, daniID, `{"name":"Otra"}`, http.StatusNotFound, httpapi.ProblemCodeNotFound},
		{"a Member of another Group", ana, ana.memberID(otherID, "Eva"), `{"name":"Otra"}`, http.StatusNotFound, httpapi.ProblemCodeNotFound},
		{"by someone outside the Group", beto, carlaID, `{"name":"Otra"}`, http.StatusNotFound, httpapi.ProblemCodeNotFound},
		{"a blank name", ana, carlaID, `{"name":"  "}`, http.StatusBadRequest, httpapi.ProblemCodeValidationFailed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := tt.who.send(http.MethodPut, "/api/v1/groups/"+groupID+"/members/"+tt.memberID, tt.body)

			decodeProblem(t, rec, tt.status, tt.code)
		})
	}
}
