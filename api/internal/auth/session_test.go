package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// whoAmI runs a request with token's session cookie (none if token is
// empty) through Authenticate and RequireUser, and returns the response and
// the User the handler saw.
func (f *fixture) whoAmI(t *testing.T, token string) (*http.Response, store.User) {
	t.Helper()
	var seen store.User
	h := f.svc.Authenticate(auth.RequireUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = auth.CurrentUser(r.Context())
	})))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	if token != "" {
		req.AddCookie(auth.SessionCookie(token))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Result(), seen
}

func TestTheSessionCookieIdentifiesTheUser(t *testing.T) {
	f := newFixture(t)
	signIn := f.signInWithCode(t, anaEmail, anaIP)

	resp, user := f.whoAmI(t, signIn.Token)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if user.ID != signIn.User.ID || user.Email != anaEmail {
		t.Errorf("CurrentUser = %+v, want %s", user, anaEmail)
	}
}

func TestTheSessionCookieIsHiddenFromScriptsAndOtherSites(t *testing.T) {
	c := auth.SessionCookie("token")

	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("HttpOnly=%v Secure=%v SameSite=%v, want HttpOnly, Secure and Lax", c.HttpOnly, c.Secure, c.SameSite)
	}
	if c.Path != "/" || c.Domain != "" {
		t.Errorf("Path=%q Domain=%q, want the whole origin and no Domain", c.Path, c.Domain)
	}
	if c.MaxAge != 90*24*60*60 {
		t.Errorf("MaxAge = %d, want 90 days", c.MaxAge)
	}
}

func TestRequestsWithoutAValidSessionAreUnauthorized(t *testing.T) {
	f := newFixture(t)
	f.signInWithCode(t, anaEmail, anaIP)

	for name, token := range map[string]string{"no cookie": "", "unknown token": "bm90LWEtcmVhbC10b2tlbg"} {
		if resp, _ := f.whoAmI(t, token); resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, resp.StatusCode)
		}
	}
}

func TestASessionExpiresAfterNinetyDaysUnused(t *testing.T) {
	f := newFixture(t)
	signIn := f.signInWithCode(t, anaEmail, anaIP)

	f.clock.Advance(90 * 24 * time.Hour)
	if resp, _ := f.whoAmI(t, signIn.Token); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status after 90 days = %d, want 401", resp.StatusCode)
	}
}

func TestUsingASessionExtendsIt(t *testing.T) {
	f := newFixture(t)
	signIn := f.signInWithCode(t, anaEmail, anaIP)

	f.clock.Advance(60 * 24 * time.Hour)
	resp, _ := f.whoAmI(t, signIn.Token)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status after 60 days = %d, want 200", resp.StatusCode)
	}
	if c := cookieNamed(resp, auth.SessionCookieName); c == nil || c.Value != signIn.Token || c.MaxAge != 90*24*60*60 {
		t.Errorf("the response should renew the cookie for another 90 days, got %v", c)
	}

	f.clock.Advance(60 * 24 * time.Hour)
	if resp, _ := f.whoAmI(t, signIn.Token); resp.StatusCode != http.StatusOK {
		t.Errorf("status 60 days after the last use = %d, want 200", resp.StatusCode)
	}
}

func cookieNamed(resp *http.Response, name string) *http.Cookie {
	for _, c := range resp.Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

func TestADeadSessionCookieIsDeleted(t *testing.T) {
	f := newFixture(t)
	signIn := f.signInWithCode(t, anaEmail, anaIP)
	f.clock.Advance(90 * 24 * time.Hour)

	resp, _ := f.whoAmI(t, signIn.Token)

	if c := cookieNamed(resp, auth.SessionCookieName); c == nil || c.MaxAge >= 0 {
		t.Errorf("the response should delete the expired cookie, got %v", c)
	}
}

func TestADeletedUsersSessionsStopWorking(t *testing.T) {
	f := newFixture(t)
	signIn := f.signInWithCode(t, anaEmail, anaIP)

	// Account deletion lives elsewhere; mark the User as deleted directly.
	if _, err := f.db.Exec("UPDATE users SET deleted_at = ? WHERE id = ?", f.clock.Now(), signIn.User.ID); err != nil {
		t.Fatal(err)
	}

	if resp, _ := f.whoAmI(t, signIn.Token); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}
