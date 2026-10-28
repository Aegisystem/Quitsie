package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
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
	outbox  *outbox
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
	box := &outbox{}
	svc, err := auth.New(auth.Config{DB: db, Mail: box, DefaultCurrency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	server := httpapi.NewServer(db, "test").WithDeps(httpapi.Deps{DB: db})
	return &recordsAPI{t: t, db: db, auth: svc, outbox: box, handler: svc.Authenticate(httpapi.NewHandler(server, builtPWA))}
}

// person is a signed-in User.
type person struct {
	user  store.User
	token string
}

// signIn signs in with a login code, as the PWA does, and returns the session.
func (a *recordsAPI) signIn(email string) person {
	a.t.Helper()
	ctx := context.Background()
	if err := a.auth.RequestCode(ctx, email, "203.0.113.7"); err != nil {
		a.t.Fatal(err)
	}
	in, err := a.auth.VerifyCode(ctx, email, a.outbox.lastCode(a.t, email), "test")
	if err != nil {
		a.t.Fatal(err)
	}
	return person{user: in.User, token: in.Token}
}

// do sends a request as p, or without a session if p is the zero person.
func (a *recordsAPI) do(p person, method, target string, body any) *httptest.ResponseRecorder {
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

// decode checks the status of rec and decodes its JSON body.
func decode[T any](t *testing.T, rec *httptest.ResponseRecorder, status int) T {
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

// trip is a Group in USD: Ana and Beto are linked Members, Carla is pending
// and Dani already left. Eva has an account but is not in it.
type trip struct {
	id                         string
	ana, beto, eva             person
	anaM, betoM, carlaM, daniM string
}

func (a *recordsAPI) seedTrip() trip {
	a.t.Helper()
	return a.seedTripOf(a.signIn("ana@example.com"), a.signIn("beto@example.com"), a.signIn("eva@example.com"))
}

// seedTripOf creates another trip for the same people.
func (a *recordsAPI) seedTripOf(ana, beto, eva person) trip {
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

	got := decode[httpapi.RecordResults](t, a.do(tr.ana, http.MethodPost, "/api/v1/records", expenses(dinner)), http.StatusOK)

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
func (a *recordsAPI) push(t *testing.T, p person, batch httpapi.RecordBatch) []httpapi.RecordResult {
	t.Helper()
	return decode[httpapi.RecordResults](t, a.do(p, http.MethodPost, "/api/v1/records", batch), http.StatusOK).Results
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

func (a *recordsAPI) seedFriendship(x, y person) friendship {
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
			p  person
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
