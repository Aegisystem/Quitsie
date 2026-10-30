package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
	"github.com/Aegisystem/Quitsie/api/internal/mail"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// recordsAPI is the whole API on a real SQLite database, behind the real
// session middleware, as the router will serve it.
type recordsAPI struct {
	t       *testing.T
	db      *sql.DB
	auth    *auth.Service
	codeBox *codeBox
	handler http.Handler
}

func newRecordsAPI(t *testing.T) *recordsAPI {
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
	box := &codeBox{}
	svc, err := auth.New(auth.Config{DB: db, Mail: box, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	server := httpapi.NewServer(db, "test").WithDeps(httpapi.Deps{DB: db})
	return &recordsAPI{t: t, db: db, auth: svc, codeBox: box, handler: svc.Authenticate(httpapi.NewHandler(server, builtPWA))}
}

// testOrigin is the app's own origin in these tests.
const testOrigin = "https://quitsie.test"

// signedIn is a signed-in User.
type signedIn struct {
	user  store.User
	token string
}

// signIn signs in with a login code, as the PWA does, and returns the session.
func (a *recordsAPI) signIn(email string) signedIn {
	a.t.Helper()
	ctx := context.Background()
	if err := a.auth.RequestCode(ctx, email, "203.0.113.7"); err != nil {
		a.t.Fatal(err)
	}
	in, err := a.auth.VerifyCode(ctx, email, a.codeBox.lastCode(a.t, email), "test")
	if err != nil {
		a.t.Fatal(err)
	}
	return signedIn{user: in.User, token: in.Token}
}

// do sends a request as p, or without a session if p is the zero value.
func (a *recordsAPI) do(p signedIn, method, target string, body any) *httptest.ResponseRecorder {
	a.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			a.t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, target, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Origin", testOrigin)
	if p.token != "" {
		req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: p.token})
	}
	rec := httptest.NewRecorder()
	a.handler.ServeHTTP(rec, req)
	return rec
}

// write runs fn with store.Write, to set up what the test needs.
func (a *recordsAPI) write(fn func(q *store.Queries, version int64) error) int64 {
	a.t.Helper()
	version, err := store.Write(context.Background(), a.db, fn)
	if err != nil {
		a.t.Fatal(err)
	}
	return version
}

// decodeBody checks the status of rec and decodes its JSON body.
func decodeBody[T any](t *testing.T, rec *httptest.ResponseRecorder, status int) T {
	t.Helper()
	var v T
	if rec.Code != status {
		t.Fatalf("code = %d, want %d (body %s)", rec.Code, status, rec.Body)
	}
	if err := json.NewDecoder(rec.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// codeBox is a mail.Sender that keeps the emails instead of sending them.
type codeBox struct {
	mu   sync.Mutex
	sent []mail.Message
}

func (o *codeBox) Send(_ context.Context, m mail.Message) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.sent = append(o.sent, m)
	return nil
}

var loginCodeDigits = regexp.MustCompile(`\b\d{6}\b`)

// lastCode returns the login code in the last email sent to `to`.
func (o *codeBox) lastCode(t *testing.T, to string) string {
	t.Helper()
	o.mu.Lock()
	defer o.mu.Unlock()
	for i := len(o.sent) - 1; i >= 0; i-- {
		if o.sent[i].To == to {
			if code := loginCodeDigits.FindString(o.sent[i].Text); code != "" {
				return code
			}
		}
	}
	t.Fatalf("no login code sent to %s", to)
	return ""
}

// trip is a Group in USD: Ana and Beto are linked Members, Carla is pending
// and Dani already left. Eva has an account but is not in it.
type trip struct {
	id                         string
	ana, beto, eva             signedIn
	anaM, betoM, carlaM, daniM string
}

func (a *recordsAPI) seedTrip() trip {
	a.t.Helper()
	return a.seedTripOf(a.signIn("ana@example.com"), a.signIn("beto@example.com"), a.signIn("eva@example.com"))
}

// seedTripOf creates another trip for the same people.
func (a *recordsAPI) seedTripOf(ana, beto, eva signedIn) trip {
	a.t.Helper()
	ctx := context.Background()
	tr := trip{
		id:  uuid.NewString(),
		ana: ana, beto: beto, eva: eva,
		anaM: uuid.NewString(), betoM: uuid.NewString(), carlaM: uuid.NewString(), daniM: uuid.NewString(),
	}
	usd := "USD"
	a.write(func(q *store.Queries, version int64) error {
		err := q.CreateLedger(ctx, store.CreateLedgerParams{
			ID: tr.id, Kind: "group", Name: "Viaje a Cartagena", Currency: &usd,
			SimplifyDebts: true, Version: version, CreatedAt: time.Now(),
		})
		if err != nil {
			return err
		}
		for _, m := range []struct {
			id, name string
			user     *string
		}{
			{tr.anaM, "Ana", &tr.ana.user.ID},
			{tr.betoM, "Beto", &tr.beto.user.ID},
			{tr.carlaM, "Carla", nil},
			{tr.daniM, "Dani", nil},
		} {
			err := q.CreateMember(ctx, store.CreateMemberParams{
				ID: m.id, LedgerID: tr.id, UserID: m.user, Name: m.name, Version: version, CreatedAt: time.Now(),
			})
			if err != nil {
				return err
			}
		}
		_, err = q.MarkMemberFormer(ctx, store.MarkMemberFormerParams{ID: tr.daniM, Version: version})
		return err
	})
	return tr
}

func id(s string) openapi_types.UUID { return uuid.MustParse(s) }

func day(s string) openapi_types.Date {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return openapi_types.Date{Time: t}
}

// dinner is 30 USD that Ana paid and split equally with Beto and Carla.
func (tr trip) dinner() httpapi.NewExpense {
	return httpapi.NewExpense{
		Id: id(uuid.NewString()), LedgerId: id(tr.id), Description: "Cena", Date: day("2026-10-25"),
		Currency: "USD", Total: 3000, SplitMethod: httpapi.Equal,
		PaidShares: []httpapi.PaidShare{{MemberId: id(tr.anaM), Amount: 3000}},
		OwedShares: []httpapi.OwedShare{
			{MemberId: id(tr.anaM), Amount: 1000},
			{MemberId: id(tr.betoM), Amount: 1000},
			{MemberId: id(tr.carlaM), Amount: 1000},
		},
	}
}

func expenses(e ...httpapi.NewExpense) httpapi.RecordBatch {
	return httpapi.RecordBatch{Expenses: e, Settlements: []httpapi.NewSettlement{}}
}

func TestPushRecordsAcceptsANewExpense(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	dinner := tr.dinner()

	got := decodeBody[httpapi.RecordResults](t, a.do(tr.ana, http.MethodPost, "/api/v1/records", expenses(dinner)), http.StatusOK)

	if len(got.Results) != 1 {
		t.Fatalf("results = %+v, want one", got.Results)
	}
	r := got.Results[0]
	if r.Id != dinner.Id || r.Kind != httpapi.RecordResultKindExpense || r.Status != httpapi.Accepted || r.Reason != nil {
		t.Errorf("result = %+v, want the dinner accepted", r)
	}
	if r.Version == nil || *r.Version == 0 {
		t.Errorf("version = %v, want the version it was stored with", r.Version)
	}
}

// push sends batch as p and returns its results.
func (a *recordsAPI) push(t *testing.T, p signedIn, batch httpapi.RecordBatch) []httpapi.RecordResult {
	t.Helper()
	return decodeBody[httpapi.RecordResults](t, a.do(p, http.MethodPost, "/api/v1/records", batch), http.StatusOK).Results
}

func TestPushRecordsAcceptsAResentExpenseWithoutStoringItTwice(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	dinner := tr.dinner()
	first := a.push(t, tr.ana, expenses(dinner))[0]

	resent := dinner
	resent.Description = "Cena (otra vez)"
	got := a.push(t, tr.ana, expenses(resent))[0]

	if got.Status != httpapi.Accepted || got.Version == nil || *got.Version != *first.Version {
		t.Errorf("resent result = %+v, want accepted with the first version %d", got, *first.Version)
	}
}

// friendship is the ledger of a Friendship between two signed-in Users.
type friendship struct {
	id     string
	aM, bM string
}

func (a *recordsAPI) seedFriendship(x, y signedIn) friendship {
	a.t.Helper()
	ctx := context.Background()
	f := friendship{id: uuid.NewString(), aM: uuid.NewString(), bM: uuid.NewString()}
	a.write(func(q *store.Queries, version int64) error {
		err := q.CreateLedger(ctx, store.CreateLedgerParams{ID: f.id, Kind: "friendship", SimplifyDebts: true, Version: version, CreatedAt: time.Now()})
		if err != nil {
			return err
		}
		for _, m := range []struct {
			id string
			p  signedIn
		}{{f.aM, x}, {f.bM, y}} {
			err := q.CreateMember(ctx, store.CreateMemberParams{
				ID: m.id, LedgerID: f.id, UserID: &m.p.user.ID, Name: m.p.user.Email, Version: version, CreatedAt: time.Now(),
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return f
}

// lunch is 2000 yen that the first Friend paid for the second.
func (f friendship) lunch() httpapi.NewExpense {
	return httpapi.NewExpense{
		Id: id(uuid.NewString()), LedgerId: id(f.id), Description: "Almuerzo", Date: day("2026-10-26"),
		Currency: "JPY", Total: 2000, SplitMethod: httpapi.Exact,
		PaidShares: []httpapi.PaidShare{{MemberId: id(f.aM), Amount: 2000}},
		OwedShares: []httpapi.OwedShare{{MemberId: id(f.bM), Amount: 2000}},
	}
}

// assertRejected checks that r rejects the record id with reason.
func assertRejected(t *testing.T, r httpapi.RecordResult, recordID openapi_types.UUID, reason httpapi.RecordResultReason) {
	t.Helper()
	if r.Id != recordID || r.Status != httpapi.Rejected || r.Reason == nil || *r.Reason != reason || r.Version != nil {
		t.Errorf("result = %+v, want %s rejected with %s", r, recordID, reason)
	}
}

func TestPushRecordsRejectsAnIDThatBelongsToAnotherRecord(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	f := a.seedFriendship(tr.ana, tr.beto)
	dinner := tr.dinner()
	a.push(t, tr.ana, expenses(dinner))

	t.Run("by another author", func(t *testing.T) {
		got := a.push(t, tr.beto, expenses(dinner))[0]
		assertRejected(t, got, dinner.Id, httpapi.RecordResultReasonIdConflict)
	})
	t.Run("in another ledger", func(t *testing.T) {
		lunch := f.lunch()
		lunch.Id = dinner.Id
		got := a.push(t, tr.ana, expenses(lunch))[0]
		assertRejected(t, got, dinner.Id, httpapi.RecordResultReasonIdConflict)
	})
}

func TestPushRecordsRejectsLedgersTheUserCannotUse(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	deleted := a.seedTripOf(tr.ana, tr.beto, tr.eva)
	a.write(func(q *store.Queries, version int64) error {
		_, err := q.DeleteLedger(context.Background(), store.DeleteLedgerParams{ID: deleted.id, DeletedAt: ptr(time.Now()), Version: version})
		return err
	})

	t.Run("not a Member", func(t *testing.T) {
		dinner := tr.dinner()
		assertRejected(t, a.push(t, tr.eva, expenses(dinner))[0], dinner.Id, httpapi.RecordResultReasonLedgerUnavailable)
	})
	t.Run("no such ledger", func(t *testing.T) {
		dinner := tr.dinner()
		dinner.LedgerId = id(uuid.NewString())
		assertRejected(t, a.push(t, tr.ana, expenses(dinner))[0], dinner.Id, httpapi.RecordResultReasonLedgerUnavailable)
	})
	t.Run("deleted Group", func(t *testing.T) {
		dinner := deleted.dinner()
		assertRejected(t, a.push(t, tr.ana, expenses(dinner))[0], dinner.Id, httpapi.RecordResultReasonLedgerUnavailable)
	})
	t.Run("former member", func(t *testing.T) {
		a.write(func(q *store.Queries, version int64) error {
			_, err := q.MarkMemberFormer(context.Background(), store.MarkMemberFormerParams{ID: tr.betoM, Version: version})
			return err
		})
		dinner := tr.dinner()
		dinner.PaidShares[0].MemberId = id(tr.betoM)
		assertRejected(t, a.push(t, tr.beto, expenses(dinner))[0], dinner.Id, httpapi.RecordResultReasonLedgerUnavailable)
	})
}

func ptr[T any](v T) *T { return &v }

func TestPushRecordsRejectsExpensesThatBreakTheRules(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	tests := []struct {
		name   string
		change func(e *httpapi.NewExpense)
		want   httpapi.RecordResultReason
	}{
		{"a currency other than the Group's", func(e *httpapi.NewExpense) { e.Currency = "EUR" }, httpapi.RecordResultReasonCurrencyMismatch},
		{"a former member", func(e *httpapi.NewExpense) { e.OwedShares[2].MemberId = id(tr.daniM) }, httpapi.RecordResultReasonInvalidParticipants},
		{"someone from outside the Group", func(e *httpapi.NewExpense) { e.OwedShares[2].MemberId = id(uuid.NewString()) }, httpapi.RecordResultReasonInvalidParticipants},
		{"the same Participant twice", func(e *httpapi.NewExpense) { e.OwedShares[2].MemberId = id(tr.betoM) }, httpapi.RecordResultReasonInvalidParticipants},
		{"shares that do not add up", func(e *httpapi.NewExpense) { e.OwedShares[2].Amount = 999 }, httpapi.RecordResultReasonInvalidAmounts},
		{"a total over 10^15", func(e *httpapi.NewExpense) {
			e.Total, e.PaidShares[0].Amount, e.OwedShares[0].Amount = 1_000_000_000_000_998, 1_000_000_000_000_998, 999_999_999_999_998
		}, httpapi.RecordResultReasonInvalidAmounts},
		{"no paid shares", func(e *httpapi.NewExpense) { e.PaidShares = nil }, httpapi.RecordResultReasonInvalidAmounts},
		{"an unknown split method", func(e *httpapi.NewExpense) { e.SplitMethod = "shares" }, httpapi.RecordResultReasonInvalidRecord},
		{"percentages missing", func(e *httpapi.NewExpense) { e.SplitMethod = httpapi.Percentage }, httpapi.RecordResultReasonInvalidRecord},
		{"an empty description", func(e *httpapi.NewExpense) { e.Description = "" }, httpapi.RecordResultReasonInvalidRecord},
		{"a description over 200 characters", func(e *httpapi.NewExpense) { e.Description = strings.Repeat("ñ", 201) }, httpapi.RecordResultReasonInvalidRecord},
		{"an unknown category", func(e *httpapi.NewExpense) { e.Category = ptr(httpapi.Category("yachts")) }, httpapi.RecordResultReasonInvalidRecord},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dinner := tr.dinner()
			tt.change(&dinner)
			assertRejected(t, a.push(t, tr.ana, expenses(dinner))[0], dinner.Id, tt.want)
		})
	}
}

func TestPushRecordsAcceptsAnOwedShareRoundedToZero(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	gum := tr.dinner()
	gum.Total, gum.PaidShares[0].Amount = 2, 2
	gum.OwedShares[0].Amount, gum.OwedShares[1].Amount, gum.OwedShares[2].Amount = 1, 1, 0
	gum.Category = ptr(httpapi.Category("groceries"))

	if got := a.push(t, tr.ana, expenses(gum))[0]; got.Status != httpapi.Accepted {
		t.Errorf("result = %+v, want accepted", got)
	}
}

// payback is Beto paying Ana back 10 USD.
func (tr trip) payback() httpapi.NewSettlement {
	return httpapi.NewSettlement{
		Id: id(uuid.NewString()), LedgerId: id(tr.id), FromMemberId: id(tr.betoM), ToMemberId: id(tr.anaM),
		Amount: 1000, Currency: "USD", Date: day("2026-10-26"),
	}
}

func settlements(s ...httpapi.NewSettlement) httpapi.RecordBatch {
	return httpapi.RecordBatch{Expenses: []httpapi.NewExpense{}, Settlements: s}
}

func TestPushRecordsAcceptsSettlementsIdempotently(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	payback := tr.payback()

	first := a.push(t, tr.beto, settlements(payback))[0]
	again := a.push(t, tr.beto, settlements(payback))[0]

	if first.Id != payback.Id || first.Kind != httpapi.RecordResultKindSettlement || first.Status != httpapi.Accepted || first.Version == nil {
		t.Fatalf("result = %+v, want the payback accepted", first)
	}
	if again.Status != httpapi.Accepted || again.Version == nil || *again.Version != *first.Version {
		t.Errorf("resent result = %+v, want accepted with version %d", again, *first.Version)
	}
}

func TestPushRecordsRejectsSettlementsThatBreakTheRules(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	taken := tr.payback()
	a.push(t, tr.beto, settlements(taken))
	tests := []struct {
		name   string
		by     signedIn
		change func(s *httpapi.NewSettlement)
		want   httpapi.RecordResultReason
	}{
		{"an id of another author", tr.ana, func(s *httpapi.NewSettlement) { s.Id = taken.Id }, httpapi.RecordResultReasonIdConflict},
		{"a ledger the User is not in", tr.eva, func(*httpapi.NewSettlement) {}, httpapi.RecordResultReasonLedgerUnavailable},
		{"a currency other than the Group's", tr.beto, func(s *httpapi.NewSettlement) { s.Currency = "EUR" }, httpapi.RecordResultReasonCurrencyMismatch},
		{"a former member", tr.beto, func(s *httpapi.NewSettlement) { s.ToMemberId = id(tr.daniM) }, httpapi.RecordResultReasonInvalidParticipants},
		{"paying oneself", tr.beto, func(s *httpapi.NewSettlement) { s.ToMemberId = id(tr.betoM) }, httpapi.RecordResultReasonInvalidParticipants},
		{"an amount over 10^15", tr.beto, func(s *httpapi.NewSettlement) { s.Amount = 1_000_000_000_000_001 }, httpapi.RecordResultReasonInvalidAmounts},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tr.payback()
			tt.change(&s)
			assertRejected(t, a.push(t, tt.by, settlements(s))[0], s.Id, tt.want)
		})
	}
}

func TestPushRecordsNeedsASession(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()

	rec := a.do(signedIn{}, http.MethodPost, "/api/v1/records", expenses(tr.dinner()))

	decodeProblem(t, rec, http.StatusUnauthorized, httpapi.ProblemCodeUnauthenticated)
}

func TestPushRecordsTakesAtMostAHundredOfEach(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	batch := expenses()
	for range 101 {
		batch.Expenses = append(batch.Expenses, tr.dinner())
	}

	decodeProblem(t, a.do(tr.ana, http.MethodPost, "/api/v1/records", batch), http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
}

// pull asks for the changes after since as p.
func (a *recordsAPI) pull(t *testing.T, p signedIn, since int64) httpapi.Changes {
	t.Helper()
	return decodeBody[httpapi.Changes](t, a.do(p, http.MethodGet, fmt.Sprintf("/api/v1/sync?since=%d", since), nil), http.StatusOK)
}

func TestPullChangesBringsWhatAnotherMemberPushed(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	dinner := tr.dinner()
	dinner.Category = ptr(httpapi.Category("food"))
	dinner.SplitMethod = httpapi.Percentage
	dinner.OwedShares[0].SplitValue, dinner.OwedShares[1].SplitValue, dinner.OwedShares[2].SplitValue = ptr[int64](3334), ptr[int64](3333), ptr[int64](3333)
	pushed := a.push(t, tr.ana, expenses(dinner))[0]

	got := a.pull(t, tr.beto, 0)

	if got.HasMore || got.Cursor != *pushed.Version {
		t.Errorf("cursor = %d, hasMore = %v, want %d and false", got.Cursor, got.HasMore, *pushed.Version)
	}
	if len(got.Groups) != 1 || got.Groups[0].Id != id(tr.id) || got.Groups[0].Name != "Viaje a Cartagena" || got.Groups[0].Currency != "USD" || !got.Groups[0].SimplifyDebts {
		t.Errorf("groups = %+v, want the trip", got.Groups)
	}
	if len(got.Members) != 4 {
		t.Errorf("members = %+v, want all four, the former one too", got.Members)
	}
	if len(got.Expenses) != 1 {
		t.Fatalf("expenses = %+v, want the dinner", got.Expenses)
	}
	e := got.Expenses[0]
	if e.Id != dinner.Id || e.LedgerId != dinner.LedgerId || e.Description != "Cena" || e.Date != dinner.Date ||
		e.Currency != "USD" || e.Total != 3000 || e.SplitMethod != httpapi.Percentage || e.CreatedBy != id(tr.ana.user.ID) ||
		e.Version != *pushed.Version || e.DeletedAt != nil || e.CreatedAt.IsZero() || e.Category == nil || *e.Category != "food" {
		t.Errorf("expense = %+v, want the dinner as pushed", e)
	}
	byMember := func(a, b httpapi.OwedShare) int { return strings.Compare(a.MemberId.String(), b.MemberId.String()) }
	slices.SortFunc(e.OwedShares, byMember)
	slices.SortFunc(dinner.OwedShares, byMember)
	if !reflect.DeepEqual(e.PaidShares, dinner.PaidShares) || !reflect.DeepEqual(e.OwedShares, dinner.OwedShares) {
		t.Errorf("shares = %+v and %+v, want %+v and %+v", e.PaidShares, e.OwedShares, dinner.PaidShares, dinner.OwedShares)
	}
}

func TestPullChangesBringsPushedSettlementsAndFriendships(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	f := a.seedFriendship(tr.ana, tr.beto)
	payback := tr.payback()
	a.push(t, tr.beto, settlements(payback))

	got := a.pull(t, tr.ana, 0)

	if len(got.Friendships) != 1 || got.Friendships[0].Id != id(f.id) {
		t.Errorf("friendships = %+v, want the Friendship with Beto", got.Friendships)
	}
	if len(got.Settlements) != 1 {
		t.Fatalf("settlements = %+v, want the payback", got.Settlements)
	}
	s := got.Settlements[0]
	if s.Id != payback.Id || s.LedgerId != payback.LedgerId || s.FromMemberId != payback.FromMemberId || s.ToMemberId != payback.ToMemberId ||
		s.Amount != 1000 || s.Currency != "USD" || s.Date != payback.Date || s.CreatedBy != id(tr.beto.user.ID) || s.DeletedAt != nil {
		t.Errorf("settlement = %+v, want the payback as pushed", s)
	}
}

func TestPushRecordsLeavesOneActivityPerStoredRecord(t *testing.T) {
	a := newRecordsAPI(t)
	tr := a.seedTrip()
	dinner, payback := tr.dinner(), tr.payback()
	broken := tr.dinner()
	broken.Total = 1
	batch := httpapi.RecordBatch{Expenses: []httpapi.NewExpense{dinner, broken}, Settlements: []httpapi.NewSettlement{payback}}
	a.push(t, tr.ana, batch)
	a.push(t, tr.ana, batch) // resending stores nothing new

	got := a.pull(t, tr.beto, 0).Activities

	if len(got) != 2 {
		t.Fatalf("activities = %+v, want one for the dinner and one for the payback", got)
	}
	d, s := got[0], got[1]
	if d.Kind != httpapi.ExpenseCreated || d.LedgerId != id(tr.id) || d.UserId != id(tr.ana.user.ID) || d.CreatedAt.IsZero() {
		t.Errorf("first activity = %+v, want Ana creating the dinner", d)
	}
	wantDetails := map[string]any{"expenseId": dinner.Id.String(), "description": "Cena", "total": float64(3000), "currency": "USD"}
	if !reflect.DeepEqual(d.Details, wantDetails) {
		t.Errorf("details = %+v, want %+v", d.Details, wantDetails)
	}
	if s.Kind != httpapi.SettlementCreated || s.UserId != id(tr.ana.user.ID) {
		t.Errorf("second activity = %+v, want Ana recording the payback", s)
	}
	wantDetails = map[string]any{
		"settlementId": payback.Id.String(), "fromMemberId": tr.betoM, "toMemberId": tr.anaM, "amount": float64(1000), "currency": "USD",
	}
	if !reflect.DeepEqual(s.Details, wantDetails) {
		t.Errorf("details = %+v, want %+v", s.Details, wantDetails)
	}
}
