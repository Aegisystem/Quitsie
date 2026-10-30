// Package authtest helps tests sign in with Google without reaching it.
package authtest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/coreos/go-oidc/v3/oidc/oidctest"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
)

const (
	googleClientID     = "quitsie-test.apps.googleusercontent.com"
	googleClientSecret = "test-secret"
	googleKeyID        = "test-key"
)

// GoogleAccount is who signs in at the fake Google.
type GoogleAccount struct {
	Sub           string
	Email         string
	EmailVerified bool
	Name          string
}

// grant is an authorization code the fake Google handed out.
type grant struct {
	challenge   string
	redirectURI string
	nonce       string
	account     GoogleAccount
}

// FakeGoogle is an OpenID Connect provider that behaves like Google's for
// the authorization code flow with PKCE. oidctest serves discovery and the
// signing keys; FakeGoogle adds the token endpoint and plays the consent
// screen.
type FakeGoogle struct {
	// URL is the issuer.
	URL string
	// Now is the time on the provider's clock.
	Now func() time.Time

	oidc        *oidctest.Server
	key         *rsa.PrivateKey
	redirectURL string

	mu     sync.Mutex
	grants map[string]grant
	// Tamper, when set, edits the ID token's claims before signing.
	Tamper func(claims map[string]any)
	// SignWith, when set, signs ID tokens with a key Google does not publish.
	SignWith *rsa.PrivateKey
}

// NewFakeGoogle starts a provider that knows one client, whose callback is
// redirectURL.
func NewFakeGoogle(t *testing.T, redirectURL string, now func() time.Time) *FakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	g := &FakeGoogle{
		oidc: &oidctest.Server{
			PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: googleKeyID, Algorithm: oidc.RS256}},
		},
		key:         key,
		Now:         now,
		redirectURL: redirectURL,
		grants:      make(map[string]grant),
	}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	g.URL = srv.URL
	g.oidc.SetIssuer(srv.URL)
	return g
}

// Config is the Service's configuration to sign in with this provider.
func (g *FakeGoogle) Config() *auth.GoogleConfig {
	return &auth.GoogleConfig{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURL:  g.redirectURL,
		IssuerURL:    g.URL,
	}
}

func (g *FakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		g.serveToken(w, r)
		return
	}
	g.oidc.ServeHTTP(w, r)
}

// Consent plays the person approving Quitsie on Google's consent screen:
// it checks the authorization request and returns the URL Google would
// send the browser back to.
func (g *FakeGoogle) Consent(t *testing.T, authURL string, account GoogleAccount) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if got := u.Scheme + "://" + u.Host + u.Path; got != g.URL+"/auth" {
		t.Fatalf("authorization endpoint = %s, want %s/auth", got, g.URL)
	}
	checks := map[string]bool{
		"client_id":             q.Get("client_id") == googleClientID,
		"redirect_uri":          q.Get("redirect_uri") == g.redirectURL,
		"response_type":         q.Get("response_type") == "code",
		"scope":                 hasScopes(q.Get("scope"), "openid", "email", "profile"),
		"code_challenge_method": q.Get("code_challenge_method") == "S256",
		"code_challenge":        q.Get("code_challenge") != "",
		"state":                 q.Get("state") != "",
		"nonce":                 q.Get("nonce") != "",
	}
	for param, ok := range checks {
		if !ok {
			t.Fatalf("authorization request has a bad %s: %s", param, authURL)
		}
	}

	code := rand.Text()
	g.mu.Lock()
	g.grants[code] = grant{
		challenge:   q.Get("code_challenge"),
		redirectURI: q.Get("redirect_uri"),
		nonce:       q.Get("nonce"),
		account:     account,
	}
	g.mu.Unlock()
	return q.Get("redirect_uri") + "?" + url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
}

func hasScopes(scope string, want ...string) bool {
	got := strings.Fields(scope)
	for _, w := range want {
		if !slices.Contains(got, w) {
			return false
		}
	}
	return true
}

func (g *FakeGoogle) serveToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		tokenError(w, "invalid_request")
		return
	}
	id, secret, ok := r.BasicAuth()
	if ok {
		id, _ = url.QueryUnescape(id)
		secret, _ = url.QueryUnescape(secret)
	} else {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != googleClientID || secret != googleClientSecret {
		tokenError(w, "invalid_client")
		return
	}

	g.mu.Lock()
	gr, found := g.grants[r.PostForm.Get("code")]
	delete(g.grants, r.PostForm.Get("code")) // codes work once
	tamper, signWith := g.Tamper, g.SignWith
	g.mu.Unlock()

	verifier := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	switch {
	case r.PostForm.Get("grant_type") != "authorization_code", !found,
		r.PostForm.Get("redirect_uri") != gr.redirectURI,
		base64.RawURLEncoding.EncodeToString(verifier[:]) != gr.challenge:
		tokenError(w, "invalid_grant")
		return
	}

	now := g.Now()
	claims := map[string]any{
		"iss":            g.URL,
		"aud":            googleClientID,
		"sub":            gr.account.Sub,
		"email":          gr.account.Email,
		"email_verified": gr.account.EmailVerified,
		"name":           gr.account.Name,
		"nonce":          gr.nonce,
		"iat":            now.Unix(),
		"exp":            now.Add(time.Hour).Unix(),
	}
	if tamper != nil {
		tamper(claims)
	}
	key := g.key
	if signWith != nil {
		key = signWith
	}
	raw, _ := json.Marshal(claims)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"access_token": "fake-access-token",
		"token_type":   "Bearer",
		"expires_in":   3600,
		"id_token":     oidctest.SignIDToken(key, googleKeyID, oidc.RS256, string(raw)),
	})
}

func tokenError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(map[string]string{"error": code})
}
