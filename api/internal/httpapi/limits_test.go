package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
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
	for i := range 100 {
		e := httpapi.NewExpense{
			Id: id(uuid.NewString()), LedgerId: id(tr.id), Description: strings.Repeat("🍕", 200), Date: day("2026-10-25"),
			Currency: "USD", Total: share * int64(size), SplitMethod: httpapi.Percentage,
		}
		for _, m := range members {
			e.PaidShares = append(e.PaidShares, httpapi.PaidShare{MemberId: id(m), Amount: share})
			e.OwedShares = append(e.OwedShares, httpapi.OwedShare{MemberId: id(m), Amount: share, SplitValue: &percentage})
		}
		batch.Expenses = append(batch.Expenses, e)
		// Back and forth, so that no Balance goes beyond the limit.
		from, to := tr.anaM, tr.betoM
		if i%2 == 1 {
			from, to = to, from
		}
		batch.Settlements = append(batch.Settlements, httpapi.NewSettlement{
			Id: id(uuid.NewString()), LedgerId: id(tr.id), FromMemberId: id(from), ToMemberId: id(to),
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

// Whoever signs in has not shown who they are yet, and all they send is an
// email and a code. A long crafted address once cost a minute of CPU.
func TestSigningInOnlyTakesASmallBody(t *testing.T) {
	a := newTestAPI(t)
	c := a.newClient(t)
	tooLarge := `{"email":"` + strings.Repeat("(a", 4<<10) + `@example.com","code":"123456"}`

	for _, path := range []string{"/api/v1/auth/login-code", "/api/v1/auth/login-code/verify"} {
		wantProblem(t, c.do(http.MethodPost, path, tooLarge), http.StatusRequestEntityTooLarge, httpapi.ProblemCodeBodyTooLarge)
	}

	longest := strings.Repeat("a", 240) + "@example.com"
	wantStatus(t, c.do(http.MethodPost, "/api/v1/auth/login-code", `{"email":"`+longest+`"}`), http.StatusNoContent)
}

// The limit per IP counts requests, and each can weigh 2 MiB.
func TestAUserCanOnlySendSoMuchPerHour(t *testing.T) {
	w := newWorld(t)
	ana, beto := w.signIn("Ana"), w.signIn("Beto")
	// Its name is too long, so each one costs its size and stores nothing.
	mebibyte := `{"name":"` + strings.Repeat("x", 1<<20-100) + `","currency":"COP"}`
	for i := range 5 {
		if rec := ana.send(http.MethodPost, "/api/v1/groups", mebibyte); rec.Code != http.StatusBadRequest {
			t.Fatalf("request %d: code = %d, want 400", i+1, rec.Code)
		}
	}

	rec := ana.send(http.MethodPost, "/api/v1/groups", mebibyte)

	if got := rec.Header().Get("Retry-After"); got == "" {
		t.Error("Retry-After is missing")
	}
	decodeProblem(t, rec, http.StatusTooManyRequests, httpapi.ProblemCodeRateLimited)
	if rec := ana.send(http.MethodGet, "/api/v1/me", ""); rec.Code != http.StatusOK {
		t.Errorf("reading: code = %d, want 200", rec.Code)
	}
	if rec := beto.send(http.MethodPost, "/api/v1/groups", `{"name":"Casa","currency":"COP"}`); rec.Code != http.StatusCreated {
		t.Errorf("another User: code = %d, want 201", rec.Code)
	}
	// Whatever else happens, nobody is kept from signing out.
	if rec := ana.send(http.MethodPost, "/api/v1/auth/logout", ""); rec.Code != http.StatusNoContent {
		t.Errorf("signing out: code = %d, want 204", rec.Code)
	}
}

func TestALongHistoryArrivesInPages(t *testing.T) {
	w := newWorld(t)
	ana := w.signIn("Ana")
	groupID := ana.createGroup(`{"name":"Casa","currency":"COP"}`)
	const history = 25_000
	_, err := w.db.Exec(`
		WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i + 1 FROM n WHERE i < $1)
		INSERT INTO activities (id, ledger_id, user_id, kind, details, version, created_at)
		SELECT '00000000-0000-7000-8000-' || lpad(i::text, 12, '0'), $2, $3, 'group_renamed', '{}', 1000 + i, '2026-10-31 00:00:00+00:00' FROM n`,
		history, groupID, ana.id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.db.Exec(`UPDATE version_counter SET value = $1`, 1000+history); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	since, pages := int64(0), 0
	for {
		changes := decode[httpapi.Changes](t, ana.send(http.MethodGet, fmt.Sprintf("/api/v1/sync?since=%d", since), ""), http.StatusOK)
		pages++
		if len(changes.Activities) > 20_100 {
			t.Fatalf("page %d has %d Activities, want it cut around 20,000 rows", pages, len(changes.Activities))
		}
		for _, a := range changes.Activities {
			seen[a.Id.String()] = true
		}
		since = changes.Cursor
		if !changes.HasMore {
			break
		}
	}

	if pages != 2 {
		t.Errorf("pages = %d, want 2", pages)
	}
	if want := history + 1; len(seen) != want { // and the one of creating the Group
		t.Errorf("got %d Activities, want %d", len(seen), want)
	}
}

// limited is a server with a clock the test moves.
func limited(t *testing.T) (http.Handler, *clock) {
	t.Helper()
	c := &clock{now: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
	return httpapi.NewHandler(httpapi.NewServer(fakeDB{}, "test").WithDeps(httpapi.Deps{Now: c.Now}), builtPWA), c
}

// A connection that comes straight from the Internet, and the one from
// cloudflared, which runs on the same machine.
const (
	direct = "203.0.113.9:40000"
	tunnel = "127.0.0.1:40000"
)

// visit asks for target over a connection from peer. visitor is the
// CF-Connecting-IP the request carries, if any.
func visit(h http.Handler, peer, visitor, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.RemoteAddr = peer
	if visitor != "" {
		req.Header.Set("CF-Connecting-IP", visitor)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// spend uses up the 300 requests per minute of a client, half on the API and
// half on the PWA, and fails if any of them is refused.
func spend(t *testing.T, h http.Handler, peer, visitor string) {
	t.Helper()
	for i := range 300 {
		target := "/"
		if i%2 == 0 {
			target = "/api/v1/nope"
		}
		if rec := visit(h, peer, visitor, target); rec.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d: code = 429, want it within the limit", i+1)
		}
	}
}

func TestAClientOverTheLimitGets429AndWhenToComeBack(t *testing.T) {
	h, c := limited(t)
	spend(t, h, direct, "")

	rec := visit(h, direct, "", "/api/v1/nope")

	if got := rec.Header().Get("Retry-After"); got != "1" {
		t.Errorf("Retry-After = %q, want 1", got)
	}
	decodeProblem(t, rec, http.StatusTooManyRequests, httpapi.ProblemCodeRateLimited)
	if rec := visit(h, direct, "", "/"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("PWA over the limit: code = %d, want 429", rec.Code)
	}

	c.Advance(time.Second)

	if rec := visit(h, direct, "", "/"); rec.Code != http.StatusOK {
		t.Errorf("after Retry-After: code = %d, want 200", rec.Code)
	}
}

func TestEachClientHasItsOwnLimit(t *testing.T) {
	h, _ := limited(t)
	spend(t, h, direct, "")

	if rec := visit(h, "198.51.100.7:40000", "", "/"); rec.Code != http.StatusOK {
		t.Errorf("another client: code = %d, want 200", rec.Code)
	}
}

func TestHealthIsNeverLimitedNorCounted(t *testing.T) {
	h, _ := limited(t)
	for i := range 500 {
		if rec := visit(h, direct, "", "/api/v1/health"); rec.Code != http.StatusOK {
			t.Fatalf("health check %d: code = %d, want 200", i+1, rec.Code)
		}
	}
	spend(t, h, direct, "")

	if rec := visit(h, direct, "", "/api/v1/health"); rec.Code != http.StatusOK {
		t.Errorf("health over the limit: code = %d, want 200", rec.Code)
	}
}

func TestBehindTheTunnelEachVisitorHasItsOwnLimit(t *testing.T) {
	h, _ := limited(t)
	spend(t, h, tunnel, "198.51.100.7")

	if rec := visit(h, tunnel, "198.51.100.7", "/"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("same visitor: code = %d, want 429", rec.Code)
	}
	if rec := visit(h, tunnel, "198.51.100.8", "/"); rec.Code != http.StatusOK {
		t.Errorf("another visitor: code = %d, want 200", rec.Code)
	}
}

// Only cloudflared can vouch for that header. Whoever reaches the server
// directly could write a new address on each request and never be limited.
func TestAClientOutsideTheTunnelCannotForgeItsAddress(t *testing.T) {
	h, _ := limited(t)
	for i := range 300 {
		visit(h, direct, fmt.Sprintf("198.51.100.%d", i%256), "/")
	}

	if rec := visit(h, direct, "198.51.100.200", "/"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("forged address: code = %d, want 429", rec.Code)
	}
}

func TestAnAddressThatIsNotOneFallsBackToTheConnection(t *testing.T) {
	h, _ := limited(t)
	spend(t, h, tunnel, "not an address")

	if rec := visit(h, tunnel, "another fake", "/"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("code = %d, want 429: both count as the connection", rec.Code)
	}
}

// An IPv6 customer gets a whole /64: one person can use 2^64 addresses.
func TestAnIPv6NetworkCountsAsOneClient(t *testing.T) {
	h, _ := limited(t)
	spend(t, h, tunnel, "2001:db8:1:2::1")

	if rec := visit(h, tunnel, "2001:db8:1:2:ffff::9", "/"); rec.Code != http.StatusTooManyRequests {
		t.Errorf("same /64: code = %d, want 429", rec.Code)
	}
	if rec := visit(h, tunnel, "2001:db8:1:3::1", "/"); rec.Code != http.StatusOK {
		t.Errorf("another /64: code = %d, want 200", rec.Code)
	}
}

// askForCode sends POST /auth/login-code for a new email each time, from
// peer, with visitor in CF-Connecting-IP, and returns the status.
func (w *world) askForCode(peer, visitor string) int {
	w.t.Helper()
	w.codes++
	body := fmt.Sprintf(`{"email":"user%d@example.com"}`, w.codes)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login-code", strings.NewReader(body))
	req.RemoteAddr = peer
	req.Header.Set("CF-Connecting-IP", visitor)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", testOrigin)
	rec := httptest.NewRecorder()
	w.h.ServeHTTP(rec, req)
	return rec.Code
}

// The limit on login codes per IP reads the same address.
func TestAForgedAddressDoesNotDodgeTheLimitOfLoginCodes(t *testing.T) {
	w := newWorld(t)
	for i := range 10 {
		if got := w.askForCode(direct, fmt.Sprintf("198.51.100.%d", i)); got != http.StatusNoContent {
			t.Fatalf("code %d: status = %d, want 204", i+1, got)
		}
	}

	if got := w.askForCode(direct, "198.51.100.10"); got != http.StatusTooManyRequests {
		t.Errorf("11th code from one connection: status = %d, want 429", got)
	}
}

// Each login code is an email out of the server's budget, and a customer
// often holds a /56: 256 networks of /64.
func TestLoginCodesCountAnIPv6CustomerByIts56(t *testing.T) {
	w := newWorld(t)
	for i := range 10 {
		if got := w.askForCode(tunnel, fmt.Sprintf("2001:db8:1:%x::1", i)); got != http.StatusNoContent {
			t.Fatalf("code %d: status = %d, want 204", i+1, got)
		}
	}

	if got := w.askForCode(tunnel, "2001:db8:1:ff::1"); got != http.StatusTooManyRequests {
		t.Errorf("another /64 of the same /56: status = %d, want 429", got)
	}
	if got := w.askForCode(tunnel, "2001:db8:1:100::1"); got != http.StatusNoContent {
		t.Errorf("another /56: status = %d, want 204", got)
	}
}
