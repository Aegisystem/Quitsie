package auth_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
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

// signInOn signs email in from device and returns the session token.
func (f *fixture) signInOn(t *testing.T, email, device string) auth.SignIn {
	t.Helper()
	ctx := context.Background()
	if err := f.svc.RequestCode(ctx, email, anaIP); err != nil {
		t.Fatal(err)
	}
	signIn, err := f.svc.VerifyCode(ctx, email, f.mail.lastCode(t, email), device)
	if err != nil {
		t.Fatal(err)
	}
	return signIn
}

func TestAUserSeesTheDevicesTheyAreSignedInOn(t *testing.T) {
	f := newFixture(t)
	laptop := f.signInOn(t, anaEmail, "Firefox on Linux")
	f.clock.Advance(2 * time.Hour)
	f.signInOn(t, anaEmail, "Safari on iPhone")
	f.signInOn(t, "beto@example.com", "Chrome on Android")
	f.clock.Advance(2 * time.Hour)
	f.whoAmI(t, laptop.Token) // the laptop is now the most recently used

	sessions, err := f.svc.Sessions(context.Background(), laptop.User.ID)
	if err != nil {
		t.Fatal(err)
	}

	var devices []string
	for _, s := range sessions {
		devices = append(devices, s.Device)
	}
	if want := []string{"Firefox on Linux", "Safari on iPhone"}; !slices.Equal(devices, want) {
		t.Errorf("devices = %q, want %q (most recently used first)", devices, want)
	}
}

func TestExpiredSessionsAreNotListed(t *testing.T) {
	f := newFixture(t)
	old := f.signInOn(t, anaEmail, "Firefox on Linux")
	f.clock.Advance(90 * 24 * time.Hour)
	f.signInOn(t, anaEmail, "Safari on iPhone")

	sessions, err := f.svc.Sessions(context.Background(), old.User.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Device != "Safari on iPhone" {
		t.Errorf("sessions = %+v, want only the iPhone", sessions)
	}
}

// sessionOn returns the ID of the User's session on device.
func (f *fixture) sessionOn(t *testing.T, userID, device string) string {
	t.Helper()
	sessions, err := f.svc.Sessions(context.Background(), userID)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range sessions {
		if s.Device == device {
			return s.ID
		}
	}
	t.Fatalf("no session on %s", device)
	return ""
}

func TestEndingASessionSignsThatDeviceOut(t *testing.T) {
	f := newFixture(t)
	laptop := f.signInOn(t, anaEmail, "Firefox on Linux")
	phone := f.signInOn(t, anaEmail, "Safari on iPhone")

	err := f.svc.EndSession(context.Background(), phone.User.ID, f.sessionOn(t, phone.User.ID, "Safari on iPhone"))
	if err != nil {
		t.Fatal(err)
	}

	if resp, _ := f.whoAmI(t, phone.Token); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("the phone: status = %d, want 401", resp.StatusCode)
	}
	if resp, _ := f.whoAmI(t, laptop.Token); resp.StatusCode != http.StatusOK {
		t.Errorf("the laptop: status = %d, want 200", resp.StatusCode)
	}
}

func TestNobodyCanEndSomeoneElsesSession(t *testing.T) {
	f := newFixture(t)
	ana := f.signInOn(t, anaEmail, "Firefox on Linux")
	beto := f.signInOn(t, "beto@example.com", "Chrome on Android")

	err := f.svc.EndSession(context.Background(), beto.User.ID, f.sessionOn(t, ana.User.ID, "Firefox on Linux"))
	if !errors.Is(err, auth.ErrSessionNotFound) {
		t.Errorf("err = %v, want ErrSessionNotFound", err)
	}
	if resp, _ := f.whoAmI(t, ana.Token); resp.StatusCode != http.StatusOK {
		t.Errorf("Ana's session: status = %d, want 200", resp.StatusCode)
	}
}

// To mark "this device" in the list, and to sign out of it.
func TestHandlersKnowTheCurrentSession(t *testing.T) {
	f := newFixture(t)
	f.signInOn(t, anaEmail, "Firefox on Linux")
	phone := f.signInOn(t, anaEmail, "Safari on iPhone")

	var current auth.Session
	h := f.svc.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		current, _ = auth.CurrentSession(r.Context())
	}))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sessions", nil)
	req.AddCookie(auth.SessionCookie(phone.Token))
	h.ServeHTTP(httptest.NewRecorder(), req)

	if want := f.sessionOn(t, phone.User.ID, "Safari on iPhone"); current.ID != want || current.Device != "Safari on iPhone" {
		t.Errorf("CurrentSession = %+v, want the iPhone's session %s", current, want)
	}
}

func TestExpiredSessionsAreForgotten(t *testing.T) {
	f := newFixture(t)
	ana := f.signInOn(t, anaEmail, "Firefox on Linux")
	f.clock.Advance(90 * 24 * time.Hour)

	f.signInOn(t, "beto@example.com", "Chrome on Android")

	if strings.Contains(f.dump(t, "sessions"), ana.User.ID) {
		t.Error("sessions still holds Ana's expired session")
	}
}
