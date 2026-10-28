package httpapi_test

import (
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
)

func TestSigningInWithAnEmailedCode(t *testing.T) {
	api := newTestAPI(t)
	phone := api.newClient(t)

	resp := phone.do(http.MethodPost, "/api/v1/auth/login-code", `{"email":"ana@example.com"}`)
	wantStatus(t, resp, http.StatusNoContent)
	code := api.mail.lastCode(t, "ana@example.com")

	resp = phone.do(http.MethodPost, "/api/v1/auth/login-code/verify", `{"email":"ana@example.com","code":"`+code+`"}`)
	login := decode[httpapi.LoginResult](t, resp, http.StatusOK)

	if !login.NewUser || login.User.Email != "ana@example.com" || login.User.Name != "" || login.User.PreferredCurrency != "USD" {
		t.Errorf("login = %+v, want a new User ana@example.com with no name and USD", login)
	}
	session := cookie(resp, auth.SessionCookieName)
	if session == nil || !session.HttpOnly || !session.Secure || session.SameSite != http.SameSiteLaxMode {
		t.Errorf("session cookie = %v, want an HttpOnly, Secure, SameSite=Lax cookie", session)
	}

	me := decode[httpapi.User](t, phone.do(http.MethodGet, "/api/v1/me", ""), http.StatusOK)
	if me != login.User {
		t.Errorf("/me = %+v, want %+v", me, login.User)
	}
}

// One request that reaches each operation of the contract.
var everyOperation = []operationRequest{
	{"GetHealth", http.MethodGet, "/api/v1/health", ""},
	{"RequestLoginCode", http.MethodPost, "/api/v1/auth/login-code", `{"email":"ana@example.com"}`},
	{"VerifyLoginCode", http.MethodPost, "/api/v1/auth/login-code/verify", `{"email":"ana@example.com","code":"000000"}`},
	{"StartGoogleLogin", http.MethodGet, "/api/v1/auth/google?returnTo=/groups", ""},
	{"FinishGoogleLogin", http.MethodGet, "/api/v1/auth/google/callback?code=c&state=s", ""},
	{"Logout", http.MethodPost, "/api/v1/auth/logout", ""},
	{"ListSessions", http.MethodGet, "/api/v1/auth/sessions", ""},
	{"RevokeSession", http.MethodDelete, "/api/v1/auth/sessions/" + someID, ""},
	{"GetMe", http.MethodGet, "/api/v1/me", ""},
	{"UpdateMe", http.MethodPatch, "/api/v1/me", `{"name":"Ana"}`},
	{"DeleteMe", http.MethodDelete, "/api/v1/me", ""},
	{"CreateGroup", http.MethodPost, "/api/v1/groups", "{}"},
	{"UpdateGroup", http.MethodPatch, "/api/v1/groups/" + someID, "{}"},
	{"DeleteGroup", http.MethodDelete, "/api/v1/groups/" + someID, ""},
	{"AddPendingMember", http.MethodPost, "/api/v1/groups/" + someID + "/members", "{}"},
	{"RemoveMember", http.MethodDelete, "/api/v1/groups/" + someID + "/members/" + someID, ""},
	{"UndoClaim", http.MethodDelete, "/api/v1/groups/" + someID + "/members/" + someID + "/claim", ""},
	{"LeaveGroup", http.MethodPost, "/api/v1/groups/" + someID + "/leave", ""},
	{"RegenerateGroupInviteLink", http.MethodPost, "/api/v1/groups/" + someID + "/invite-link", ""},
	{"GetInvite", http.MethodGet, "/api/v1/invites/k3J9xQ2mV7pL8wR4tY6u", ""},
	{"ClaimMember", http.MethodPost, "/api/v1/invites/k3J9xQ2mV7pL8wR4tY6u/claim", "{}"},
	{"JoinGroup", http.MethodPost, "/api/v1/invites/k3J9xQ2mV7pL8wR4tY6u/join", ""},
	{"CreateFriendship", http.MethodPost, "/api/v1/friendships", "{}"},
	{"RegenerateFriendInviteLink", http.MethodPost, "/api/v1/friendships/" + someID + "/invite-link", ""},
	{"PushRecords", http.MethodPost, "/api/v1/records", `{"expenses":[],"settlements":[]}`},
	{"UpdateExpense", http.MethodPatch, "/api/v1/expenses/" + someID, "{}"},
	{"DeleteExpense", http.MethodDelete, "/api/v1/expenses/" + someID, ""},
	{"RestoreExpense", http.MethodPost, "/api/v1/expenses/" + someID + "/restore", ""},
	{"UpdateSettlement", http.MethodPatch, "/api/v1/settlements/" + someID, "{}"},
	{"DeleteSettlement", http.MethodDelete, "/api/v1/settlements/" + someID, ""},
	{"RestoreSettlement", http.MethodPost, "/api/v1/settlements/" + someID + "/restore", ""},
	{"PullChanges", http.MethodGet, "/api/v1/sync?since=0", ""},
}

// The operations anyone can call. GetInvite shows an invitation before its
// guest signs in.
var publicOperations = []string{
	"GetHealth", "RequestLoginCode", "VerifyLoginCode", "StartGoogleLogin", "FinishGoogleLogin", "GetInvite",
}

func TestOnlyPublicOperationsAnswerWithoutASession(t *testing.T) {
	api := newTestAPI(t)
	stranger := api.newClient(t)

	for _, op := range everyOperation {
		t.Run(op.op, func(t *testing.T) {
			resp := stranger.do(op.method, op.target, op.body)

			if slices.Contains(publicOperations, op.op) {
				if resp.StatusCode == http.StatusUnauthorized {
					t.Errorf("public operation answered 401")
				}
				return
			}
			wantProblem(t, resp, http.StatusUnauthorized, httpapi.ProblemCodeUnauthenticated)
		})
	}
}

func TestEveryOperationIsInTheSessionCheck(t *testing.T) {
	ops := reflect.TypeFor[httpapi.StrictServerInterface]()
	for i := range ops.NumMethod() {
		name := ops.Method(i).Name
		if !slices.ContainsFunc(everyOperation, func(o operationRequest) bool { return o.op == name }) {
			t.Errorf("operation %s has no request in everyOperation", name)
		}
	}
}

func TestChangesMustComeFromTheAppsOwnOrigin(t *testing.T) {
	api := newTestAPI(t)
	ana, _ := api.signedIn(t, "ana@example.com")

	for _, origin := range []string{"", "null", "https://evil.example"} {
		t.Run("from "+origin, func(t *testing.T) {
			ana.origin = origin

			wantProblem(t, ana.do(http.MethodPatch, "/api/v1/me", `{"name":"Mallory"}`), http.StatusForbidden, httpapi.ProblemCodeOriginMismatch)
			// Public operations too: another site must not be able to sign
			// the browser in, nor flood someone's inbox through it.
			wantProblem(t, ana.do(http.MethodPost, "/api/v1/auth/login-code", `{"email":"beto@example.com"}`), http.StatusForbidden, httpapi.ProblemCodeOriginMismatch)
		})
	}
	if strings.Contains(api.mail.buf.String(), "beto@example.com") {
		t.Error("a request from another origin sent a login code")
	}

	ana.origin = ""
	decode[httpapi.User](t, ana.do(http.MethodGet, "/api/v1/me", ""), http.StatusOK)
}

func TestAFailingSessionLookupIsAnInternalError(t *testing.T) {
	api := newTestAPI(t)
	ana, _ := api.signedIn(t, "ana@example.com")
	api.db.Close()

	wantProblem(t, ana.do(http.MethodGet, "/api/v1/me", ""), http.StatusInternalServerError, httpapi.ProblemCodeInternalError)
}

func TestALoginCodeMustBeTheOneSentAndStillLive(t *testing.T) {
	var clk *clock
	api := newTestAPI(t, withClock(&clk))
	phone := api.newClient(t)
	verify := func(code string) *http.Response {
		return phone.do(http.MethodPost, "/api/v1/auth/login-code/verify", `{"email":"ana@example.com","code":"`+code+`"}`)
	}
	requestCode := func() string {
		wantStatus(t, phone.do(http.MethodPost, "/api/v1/auth/login-code", `{"email":"ana@example.com"}`), http.StatusNoContent)
		return api.mail.lastCode(t, "ana@example.com")
	}
	wrong := func(code string) string {
		if code == "000000" {
			return "000001"
		}
		return "000000"
	}

	code := requestCode()
	wantProblem(t, verify(wrong(code)), http.StatusBadRequest, httpapi.ProblemCodeLoginCodeInvalid)
	wantProblem(t, verify("12ab56"), http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)

	clk.Advance(10 * time.Minute)
	wantProblem(t, verify(code), http.StatusBadRequest, httpapi.ProblemCodeLoginCodeExpired)

	code = requestCode()
	for range 5 {
		wantProblem(t, verify(wrong(code)), http.StatusBadRequest, httpapi.ProblemCodeLoginCodeInvalid)
	}
	wantProblem(t, verify(code), http.StatusTooManyRequests, httpapi.ProblemCodeTooManyAttempts)
	if c := cookie(verify(code), auth.SessionCookieName); c != nil {
		t.Errorf("a failed verification set the session cookie %v", c)
	}
}

func TestLoginCodesAreLimitedPerEmail(t *testing.T) {
	var clk *clock
	api := newTestAPI(t, withClock(&clk))
	phone := api.newClient(t)
	request := func() *http.Response {
		return phone.do(http.MethodPost, "/api/v1/auth/login-code", `{"email":"ana@example.com"}`)
	}

	for range 5 {
		wantStatus(t, request(), http.StatusNoContent)
		clk.Advance(time.Minute)
	}
	resp := request()

	wantProblem(t, resp, http.StatusTooManyRequests, httpapi.ProblemCodeRateLimited)
	// The first of the 5 codes was sent 5 minutes ago; the hour ends in 55.
	if got := resp.Header.Get("Retry-After"); got != "3300" {
		t.Errorf("Retry-After = %q, want 3300 seconds", got)
	}
}

func TestALoginCodeNeedsAnEmailAddress(t *testing.T) {
	api := newTestAPI(t)
	phone := api.newClient(t)

	for _, body := range []string{`{"email":"ana"}`, `{"email":"ana@"}`, `{}`} {
		wantProblem(t, phone.do(http.MethodPost, "/api/v1/auth/login-code", body), http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
	}
}

func TestSigningOutEndsTheSessionOfThatDeviceOnly(t *testing.T) {
	api := newTestAPI(t)
	laptop, _ := api.signedIn(t, "ana@example.com")
	phone, _ := api.signedIn(t, "ana@example.com")
	copied := laptop.sessionCookie()

	resp := laptop.do(http.MethodPost, "/api/v1/auth/logout", "")

	wantStatus(t, resp, http.StatusNoContent)
	if c := cookie(resp, auth.SessionCookieName); c == nil || c.MaxAge >= 0 {
		t.Errorf("logout cookie = %v, want one that deletes the session cookie", c)
	}
	wantProblem(t, laptop.do(http.MethodGet, "/api/v1/me", ""), http.StatusUnauthorized, httpapi.ProblemCodeUnauthenticated)
	// The server ended the session too, so a copy of the cookie is useless.
	wantProblem(t, api.withSessionCookie(t, copied).do(http.MethodGet, "/api/v1/me", ""), http.StatusUnauthorized, httpapi.ProblemCodeUnauthenticated)
	decode[httpapi.User](t, phone.do(http.MethodGet, "/api/v1/me", ""), http.StatusOK)
}

func TestAUserSeesAndClosesTheirSessions(t *testing.T) {
	api := newTestAPI(t)
	laptop := api.newClient(t)
	laptop.device = "Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0"
	laptop.signIn("ana@example.com")
	phone := api.newClient(t)
	phone.device = "Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X) Safari/604.1"
	phone.signIn("ana@example.com")
	beto, _ := api.signedIn(t, "beto@example.com")

	sessions := decode[[]httpapi.Session](t, laptop.do(http.MethodGet, "/api/v1/auth/sessions", ""), http.StatusOK)

	if len(sessions) != 2 {
		t.Fatalf("sessions = %+v, want the laptop's and the phone's", sessions)
	}
	byDevice := map[string]httpapi.Session{}
	for _, s := range sessions {
		byDevice[s.Device] = s
	}
	if s, ok := byDevice[laptop.device]; !ok || !s.Current || s.CreatedAt.IsZero() || s.LastUsedAt.IsZero() {
		t.Errorf("laptop session = %+v, want the current one, with its dates", s)
	}
	phoneSession, ok := byDevice[phone.device]
	if !ok || phoneSession.Current {
		t.Errorf("phone session = %+v, want one that is not current", phoneSession)
	}

	// Nobody can close someone else's session, and it looks like it does not exist.
	wantProblem(t, beto.do(http.MethodDelete, "/api/v1/auth/sessions/"+phoneSession.Id.String(), ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
	decode[httpapi.User](t, phone.do(http.MethodGet, "/api/v1/me", ""), http.StatusOK)

	wantStatus(t, laptop.do(http.MethodDelete, "/api/v1/auth/sessions/"+phoneSession.Id.String(), ""), http.StatusNoContent)

	wantProblem(t, phone.do(http.MethodGet, "/api/v1/me", ""), http.StatusUnauthorized, httpapi.ProblemCodeUnauthenticated)
	wantProblem(t, laptop.do(http.MethodDelete, "/api/v1/auth/sessions/"+phoneSession.Id.String(), ""), http.StatusNotFound, httpapi.ProblemCodeNotFound)
	if left := decode[[]httpapi.Session](t, laptop.do(http.MethodGet, "/api/v1/auth/sessions", ""), http.StatusOK); len(left) != 1 || !left[0].Current {
		t.Errorf("sessions after closing the phone's = %+v, want only the laptop's", left)
	}
}

func TestAUserChangesTheirNameAndPreferredCurrency(t *testing.T) {
	api := newTestAPI(t)
	ana, me := api.signedIn(t, "ana@example.com")

	got := decode[httpapi.User](t, ana.do(http.MethodPatch, "/api/v1/me", `{"name":"  Ana Pérez  "}`), http.StatusOK)
	me.Name = "Ana Pérez"
	if got != me {
		t.Errorf("after changing the name: %+v, want %+v", got, me)
	}
	got = decode[httpapi.User](t, ana.do(http.MethodPatch, "/api/v1/me", `{"preferredCurrency":"COP"}`), http.StatusOK)
	me.PreferredCurrency = "COP"
	if got != me {
		t.Errorf("after changing the currency: %+v, want %+v", got, me)
	}

	for _, body := range []string{
		`{}`,
		`{"name":null}`,
		`{"name":"   "}`,
		`{"name":"` + strings.Repeat("ñ", 81) + `"}`,
		`{"preferredCurrency":"XYZ"}`,
		`{"preferredCurrency":"cop"}`,
		`{"name":"Ana","preferredCurrency":"XAU"}`,
	} {
		wantProblem(t, ana.do(http.MethodPatch, "/api/v1/me", body), http.StatusBadRequest, httpapi.ProblemCodeValidationFailed)
	}
	if got := decode[httpapi.User](t, ana.do(http.MethodGet, "/api/v1/me", ""), http.StatusOK); got != me {
		t.Errorf("after invalid changes: %+v, want it unchanged %+v", got, me)
	}
}
