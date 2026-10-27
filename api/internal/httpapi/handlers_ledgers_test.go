package httpapi_test

import (
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
	server := httpapi.NewServer(db, "test").WithDeps(httpapi.Deps{DB: db})
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
