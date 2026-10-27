package auth_test

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
)

const (
	googleClientID     = "quitsie-test.apps.googleusercontent.com"
	googleClientSecret = "test-secret"
	googleRedirectURL  = "https://quitsie.app/api/v1/auth/google/callback"
	googleKeyID        = "test-key"
)

// googleAccount is who signs in at the fake Google.
type googleAccount struct {
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
	account     googleAccount
}

// fakeGoogle is an OpenID Connect provider that behaves like Google's for
// the authorization code flow with PKCE. oidctest serves discovery and the
// signing keys; fakeGoogle adds the token endpoint and plays the consent
// screen.
type fakeGoogle struct {
	oidc *oidctest.Server
	url  string
	key  *rsa.PrivateKey
	now  func() time.Time

	mu     sync.Mutex
	grants map[string]grant
	// tamper, when set, edits the ID token's claims before signing.
	tamper func(claims map[string]any)
	// signWith, when set, signs ID tokens with a key Google does not publish.
	signWith *rsa.PrivateKey
}

func newFakeGoogle(t *testing.T, now func() time.Time) *fakeGoogle {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	g := &fakeGoogle{
		oidc: &oidctest.Server{
			PublicKeys: []oidctest.PublicKey{{PublicKey: key.Public(), KeyID: googleKeyID, Algorithm: oidc.RS256}},
		},
		key:    key,
		now:    now,
		grants: make(map[string]grant),
	}
	srv := httptest.NewServer(g)
	t.Cleanup(srv.Close)
	g.url = srv.URL
	g.oidc.SetIssuer(srv.URL)
	return g
}

func (g *fakeGoogle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/token" {
		g.serveToken(w, r)
		return
	}
	g.oidc.ServeHTTP(w, r)
}

// consent plays the person approving Quitsie on Google's consent screen:
// it checks the authorization request and returns the URL Google would
// send the browser back to.
func (g *fakeGoogle) consent(t *testing.T, authURL string, account googleAccount) string {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if got := u.Scheme + "://" + u.Host + u.Path; got != g.url+"/auth" {
		t.Fatalf("authorization endpoint = %s, want %s/auth", got, g.url)
	}
	checks := map[string]bool{
		"client_id":             q.Get("client_id") == googleClientID,
		"redirect_uri":          q.Get("redirect_uri") == googleRedirectURL,
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

func (g *fakeGoogle) serveToken(w http.ResponseWriter, r *http.Request) {
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
	tamper, signWith := g.tamper, g.signWith
	g.mu.Unlock()

	verifier := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
	switch {
	case r.PostForm.Get("grant_type") != "authorization_code", !found,
		r.PostForm.Get("redirect_uri") != gr.redirectURI,
		base64.RawURLEncoding.EncodeToString(verifier[:]) != gr.challenge:
		tokenError(w, "invalid_grant")
		return
	}

	now := g.now()
	claims := map[string]any{
		"iss":            g.url,
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
