package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
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

// testAPI is the whole server on an HTTPS test server, with a real database
// and the development mail sender, which writes login codes to a log the
// test reads. HTTPS matters: the session cookie is Secure, so a cookie jar
// only sends it back over TLS.
type testAPI struct {
	srv  *httptest.Server
	db   *sql.DB
	auth *auth.Service
	mail *mailLog
}

func newTestAPI(t *testing.T, options ...func(*auth.Config)) *testAPI {
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

	a := &testAPI{db: db, mail: &mailLog{}}
	cfg := auth.Config{
		DB:              db,
		Mail:            mail.NewLogSender(slog.New(slog.NewTextHandler(a.mail, nil))),
		DefaultCurrency: "USD",
	}
	for _, option := range options {
		option(&cfg)
	}
	if a.auth, err = auth.New(cfg); err != nil {
		t.Fatal(err)
	}

	// The listener exists before the server starts, so the handler can be
	// built with the server's own origin.
	a.srv = httptest.NewUnstartedServer(nil)
	a.srv.Config.Handler = httpapi.NewHandler(httpapi.NewServer(db, "test").WithDeps(httpapi.Deps{
		DB:      db,
		Auth:    a.auth,
		Origins: []string{a.origin()},
	}), builtPWA)
	a.srv.StartTLS()
	t.Cleanup(a.srv.Close)
	return a
}

func (a *testAPI) origin() string {
	return "https://" + a.srv.Listener.Addr().String()
}

// mailLog is the log the development mail sender writes to.
type mailLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *mailLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

var codeInSubject = regexp.MustCompile(`subject="[^"]*\b(\d{6})\b`)

// lastCode returns the login code of the last email logged for `to`.
func (l *mailLog) lastCode(t *testing.T, to string) string {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	lines := strings.Split(l.buf.String(), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.Contains(lines[i], " to="+to+" ") {
			if m := codeInSubject.FindStringSubmatch(lines[i]); m != nil {
				return m[1]
			}
			t.Fatalf("email to %s has no 6-digit code: %q", to, lines[i])
		}
	}
	t.Fatalf("no email logged for %s in %q", to, l.buf.String())
	return ""
}

// client is one browser: it keeps its own cookies and, like a browser on
// the app's own pages, sends the app's Origin with every request.
type client struct {
	t      *testing.T
	api    *testAPI
	http   *http.Client
	origin string
	device string
}

func (a *testAPI) newClient(t *testing.T) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &client{
		t:   t,
		api: a,
		http: &http.Client{
			Transport: a.srv.Client().Transport,
			Jar:       jar,
			// Redirects are part of what the tests check.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
		origin: a.origin(),
		device: "Mozilla/5.0 (Test)",
	}
}

// do sends a request and returns the response with its body already read,
// so callers never leak a connection.
func (c *client) do(method, path, body string) *http.Response {
	c.t.Helper()
	req, err := http.NewRequest(method, c.api.srv.URL+path, strings.NewReader(body))
	if err != nil {
		c.t.Fatal(err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.origin != "" {
		req.Header.Set("Origin", c.origin)
	}
	req.Header.Set("User-Agent", c.device)
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatal(err)
	}
	resp.Body = io.NopCloser(bytes.NewReader(raw))
	return resp
}

// signIn signs email in with a login code and returns the result.
func (c *client) signIn(email string) httpapi.LoginResult {
	c.t.Helper()
	wantStatus(c.t, c.do(http.MethodPost, "/api/v1/auth/login-code", `{"email":"`+email+`"}`), http.StatusNoContent)
	code := c.api.mail.lastCode(c.t, email)
	resp := c.do(http.MethodPost, "/api/v1/auth/login-code/verify", `{"email":"`+email+`","code":"`+code+`"}`)
	return readJSON[httpapi.LoginResult](c.t, resp, http.StatusOK)
}

// signedIn returns a new client already signed in as email.
func (a *testAPI) signedIn(t *testing.T, email string) (*client, httpapi.User) {
	t.Helper()
	c := a.newClient(t)
	return c, c.signIn(email).User
}

func wantStatus(t *testing.T, resp *http.Response, status int) {
	t.Helper()
	if resp.StatusCode != status {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("%s %s: status = %d, want %d (body %q)", resp.Request.Method, resp.Request.URL.Path, resp.StatusCode, status, body)
	}
}

// readJSON checks the status and decodes the JSON body.
func readJSON[T any](t *testing.T, resp *http.Response, status int) T {
	t.Helper()
	wantStatus(t, resp, status)
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

// wantProblem checks that resp is an application/problem+json body with
// the given status and code.
func wantProblem(t *testing.T, resp *http.Response, status int, code httpapi.ProblemCode) {
	t.Helper()
	wantStatus(t, resp, status)
	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var p httpapi.Problem
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Code != code || p.Status != status || p.Title == "" {
		t.Errorf("problem = %+v, want code %q and status %d with a title", p, code, status)
	}
}

// cookie returns the cookie named name that resp sets, or nil.
func cookie(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// clock is a settable time source for auth.Config.Now.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// withClock makes the API read the time from a new clock, which it returns
// through c.
func withClock(c **clock) func(*auth.Config) {
	return func(cfg *auth.Config) {
		*c = &clock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
		cfg.Now = (*c).Now
	}
}

// sessionCookie returns the session cookie the client holds, or nil.
func (c *client) sessionCookie() *http.Cookie {
	u, err := url.Parse(c.api.srv.URL)
	if err != nil {
		c.t.Fatal(err)
	}
	for _, ck := range c.http.Jar.Cookies(u) {
		if ck.Name == auth.SessionCookieName {
			return ck
		}
	}
	return nil
}

// withSessionCookie returns a new client holding a copy of ck, as someone
// who stole it would.
func (a *testAPI) withSessionCookie(t *testing.T, ck *http.Cookie) *client {
	t.Helper()
	c := a.newClient(t)
	u, err := url.Parse(a.srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c.http.Jar.SetCookies(u, []*http.Cookie{{Name: ck.Name, Value: ck.Value, Path: "/", Secure: true, HttpOnly: true}})
	return c
}

// newGroup creates a Group in USD with one Member per name, linked to the
// User in users when there is one there, and returns its id.
func (a *testAPI) newGroup(t *testing.T, names []string, users map[string]httpapi.User) string {
	t.Helper()
	groupID := uuid.NewString()
	usd := "USD"
	now := time.Now().UTC()
	_, err := store.Write(context.Background(), a.db, func(q *store.Queries, version int64) error {
		err := q.CreateLedger(context.Background(), store.CreateLedgerParams{
			ID: groupID, Kind: "group", Name: "Viaje", Currency: &usd, SimplifyDebts: true, Version: version, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		for _, name := range names {
			var userID *string
			if u, ok := users[name]; ok {
				id := u.Id.String()
				userID = &id
			}
			err := q.CreateMember(context.Background(), store.CreateMemberParams{
				ID: uuid.NewString(), LedgerID: groupID, UserID: userID, Name: name, Version: version, CreatedAt: now,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return groupID
}
