package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
)

// newGoogleFixture returns a fixture whose Service signs in with the fake Google.
func newGoogleFixture(t *testing.T) (*fixture, *fakeGoogle) {
	t.Helper()
	var g *fakeGoogle
	f := newFixture(t, func(cfg *auth.Config) {
		g = newFakeGoogle(t, cfg.Now)
		cfg.Google = &auth.GoogleConfig{
			ClientID:     googleClientID,
			ClientSecret: googleClientSecret,
			RedirectURL:  googleRedirectURL,
			IssuerURL:    g.url,
		}
	})
	return f, g
}

// googleCallback starts signing in with Google and returns the request the
// browser makes when Google sends it back, carrying the flow cookie.
func (f *fixture) googleCallback(t *testing.T, g *fakeGoogle, account googleAccount) *http.Request {
	t.Helper()
	authURL, flow, err := f.svc.BeginGoogle(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, g.consent(t, authURL, account), nil)
	req.AddCookie(flow)
	return req
}

var anaAtGoogle = googleAccount{Sub: "108", Email: "ana@gmail.com", EmailVerified: true, Name: "Ana Pérez"}

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
