package auth_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"golang.org/x/oauth2"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/auth/authtest"
)

const googleRedirectURL = "https://quitsie.app/api/v1/auth/google/callback"

// newGoogleFixture returns a fixture whose Service signs in with the fake Google.
func newGoogleFixture(t *testing.T) (*fixture, *authtest.FakeGoogle) {
	t.Helper()
	var g *authtest.FakeGoogle
	f := newFixture(t, func(cfg *auth.Config) {
		g = authtest.NewFakeGoogle(t, googleRedirectURL, cfg.Now)
		cfg.Google = g.Config()
	})
	return f, g
}

// googleCallback starts signing in with Google and returns the request the
// browser makes when Google sends it back, carrying the flow cookie.
func (f *fixture) googleCallback(t *testing.T, g *authtest.FakeGoogle, account authtest.GoogleAccount) *http.Request {
	t.Helper()
	authURL, flow, err := f.svc.BeginGoogle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, g.Consent(t, authURL, account), nil)
	req.AddCookie(flow)
	return req
}

var anaAtGoogle = authtest.GoogleAccount{Sub: "108", Email: "ana@gmail.com", EmailVerified: true, Name: "Ana Pérez"}

func TestGoogleSignsANewUserIn(t *testing.T) {
	f, g := newGoogleFixture(t)

	signIn, err := f.svc.FinishGoogle(context.Background(), f.googleCallback(t, g, anaAtGoogle), device)
	if err != nil {
		t.Fatal(err)
	}

	if !signIn.NewUser || signIn.User.Email != "ana@gmail.com" || signIn.User.Name != "Ana Pérez" {
		t.Errorf("SignIn = %+v, want a new User Ana Pérez <ana@gmail.com>", signIn)
	}
	if resp, user := f.whoAmI(t, signIn.Token); resp.StatusCode != http.StatusOK || user.ID != signIn.User.ID {
		t.Errorf("the session does not identify the new User: status %d, user %+v", resp.StatusCode, user)
	}
}

func TestGoogleSignsInTheUserWhoAlreadyHasThatEmail(t *testing.T) {
	f, g := newGoogleFixture(t)
	byCode := f.signInWithCode(t, "ana@gmail.com", anaIP)

	signIn, err := f.svc.FinishGoogle(context.Background(), f.googleCallback(t, g, anaAtGoogle), device)
	if err != nil {
		t.Fatal(err)
	}
	if signIn.User.ID != byCode.User.ID || signIn.NewUser {
		t.Errorf("SignIn = %+v, want the existing User %s", signIn, byCode.User.ID)
	}
}

func TestAGoogleAccountStaysLinkedWhenItsEmailChanges(t *testing.T) {
	f, g := newGoogleFixture(t)
	ctx := context.Background()
	first, err := f.svc.FinishGoogle(ctx, f.googleCallback(t, g, anaAtGoogle), device)
	if err != nil {
		t.Fatal(err)
	}

	renamed := anaAtGoogle
	renamed.Email = "ana.perez@gmail.com"
	again, err := f.svc.FinishGoogle(ctx, f.googleCallback(t, g, renamed), device)
	if err != nil {
		t.Fatal(err)
	}
	if again.User.ID != first.User.ID || again.NewUser {
		t.Errorf("signed in as %s (new: %v), want %s", again.User.ID, again.NewUser, first.User.ID)
	}
}

func TestTheGoogleFlowCookieIsShortLivedAndHidden(t *testing.T) {
	f, _ := newGoogleFixture(t)

	_, c, err := f.svc.BeginGoogle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Errorf("HttpOnly=%v Secure=%v SameSite=%v, want HttpOnly, Secure and Lax", c.HttpOnly, c.Secure, c.SameSite)
	}
	if c.MaxAge != 10*60 || c.Path != "/api/v1/auth/google/callback" {
		t.Errorf("MaxAge=%d Path=%q, want 10 minutes on the callback only", c.MaxAge, c.Path)
	}
}

// finishFails checks that FinishGoogle refuses req with want and starts no session.
func (f *fixture) finishFails(t *testing.T, req *http.Request, want error) {
	t.Helper()
	signIn, err := f.svc.FinishGoogle(context.Background(), req, device)
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
	if signIn.Token != "" {
		t.Error("a refused sign-in returned a session token")
	}
}

// withCookie returns req carrying only the flow cookie value.
func withCookie(req *http.Request, value string) *http.Request {
	req = req.Clone(req.Context())
	req.Header.Del("Cookie")
	if value != "" {
		req.AddCookie(&http.Cookie{Name: auth.GoogleCookieName, Value: value})
	}
	return req
}

// Login CSRF: Google's answer must come back to the browser that asked.
func TestGoogleAnswersNeedTheBrowserThatAsked(t *testing.T) {
	f, g := newGoogleFixture(t)

	t.Run("no flow cookie", func(t *testing.T) {
		f.finishFails(t, withCookie(f.googleCallback(t, g, anaAtGoogle), ""), auth.ErrGoogleFailed)
	})
	t.Run("another flow's cookie", func(t *testing.T) {
		_, other, err := f.svc.BeginGoogle(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		f.finishFails(t, withCookie(f.googleCallback(t, g, anaAtGoogle), other.Value), auth.ErrGoogleFailed)
	})
}

// PKCE: someone who intercepts the callback URL (code and state) and forges
// a flow cookie for it still lacks the verifier.
func TestAStolenGoogleCodeIsUseless(t *testing.T) {
	f, g := newGoogleFixture(t)
	victim := f.googleCallback(t, g, anaAtGoogle)

	forged := victim.URL.Query().Get("state") + "." + rand.Text() + "." + oauth2.GenerateVerifier()
	f.finishFails(t, withCookie(victim, forged), auth.ErrGoogleFailed)
}

func TestGoogleIDTokensAreVerified(t *testing.T) {
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for name, spoil := range map[string]func(g *authtest.FakeGoogle){
		"wrong nonce": func(g *authtest.FakeGoogle) { g.Tamper = func(c map[string]any) { c["nonce"] = "replayed" } },
		"another client": func(g *authtest.FakeGoogle) {
			g.Tamper = func(c map[string]any) { c["aud"] = "someone-else.apps.googleusercontent.com" }
		},
		"another issuer": func(g *authtest.FakeGoogle) { g.Tamper = func(c map[string]any) { c["iss"] = "https://evil.example" } },
		"expired": func(g *authtest.FakeGoogle) {
			g.Tamper = func(c map[string]any) { c["exp"] = g.Now().Add(-time.Minute).Unix() }
		},
		"unknown key":      func(g *authtest.FakeGoogle) { g.SignWith = otherKey },
		"unverified email": func(g *authtest.FakeGoogle) { g.Tamper = func(c map[string]any) { c["email_verified"] = false } },
	} {
		t.Run(name, func(t *testing.T) {
			f, g := newGoogleFixture(t)
			spoil(g)
			f.finishFails(t, f.googleCallback(t, g, anaAtGoogle), auth.ErrGoogleFailed)
		})
	}
}

func TestDecliningAtGoogleIsACancellation(t *testing.T) {
	f, _ := newGoogleFixture(t)
	authURL, flow, err := f.svc.BeginGoogle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	state := mustQuery(t, authURL).Get("state")

	req := httptest.NewRequest(http.MethodGet, googleRedirectURL+"?error=access_denied&state="+state, nil)
	req.AddCookie(flow)
	f.finishFails(t, req, auth.ErrGoogleCancelled)
}

func TestGoogleIsOffWithoutConfiguration(t *testing.T) {
	f := newFixture(t)

	if _, _, err := f.svc.BeginGoogle(context.Background()); !errors.Is(err, auth.ErrGoogleDisabled) {
		t.Errorf("BeginGoogle: err = %v, want ErrGoogleDisabled", err)
	}
}

func mustQuery(t *testing.T, rawURL string) url.Values {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

func TestADeletedAccountFreesItsGoogleAccount(t *testing.T) {
	f, g := newGoogleFixture(t)
	ctx := context.Background()
	first, err := f.svc.FinishGoogle(ctx, f.googleCallback(t, g, anaAtGoogle), device)
	if err != nil {
		t.Fatal(err)
	}

	if err := f.svc.DeleteUser(ctx, first.User.ID); err != nil {
		t.Fatal(err)
	}
	again, err := f.svc.FinishGoogle(ctx, f.googleCallback(t, g, anaAtGoogle), device)
	if err != nil {
		t.Fatal(err)
	}

	if !again.NewUser || again.User.ID == first.User.ID {
		t.Errorf("signing in again = %+v, want a new User, not %s", again, first.User.ID)
	}
}
