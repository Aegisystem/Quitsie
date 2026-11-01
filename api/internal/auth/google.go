package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// GoogleConfig is the OAuth client registered in Google Cloud. ClientID,
// ClientSecret and RedirectURL come from the server's configuration.
type GoogleConfig struct {
	ClientID     string
	ClientSecret string
	// RedirectURL is the callback registered in Google Cloud, such as
	// https://quitsie.app/api/v1/auth/google/callback.
	RedirectURL string
	// IssuerURL defaults to https://accounts.google.com; tests point it to a
	// fake provider.
	IssuerURL string
}

var (
	// ErrGoogleDisabled means the Service has no GoogleConfig.
	ErrGoogleDisabled = errors.New("auth: Google sign-in is not configured")
	// ErrGoogleCancelled means the person declined on Google's consent screen.
	ErrGoogleCancelled = errors.New("auth: Google sign-in cancelled")
	// ErrGoogleFailed means Google's answer could not be trusted or used:
	// the flow cookie is missing or does not match, the code exchange
	// failed, the ID token is invalid, or the email is not verified.
	ErrGoogleFailed = errors.New("auth: Google sign-in failed")
)

// GoogleEnabled reports whether the Service has a GoogleConfig, without
// which BeginGoogle and FinishGoogle return ErrGoogleDisabled.
func (s *Service) GoogleEnabled() bool {
	return s.google != nil
}

const (
	// GoogleCookieName is the cookie that keeps state, nonce and PKCE
	// verifier while the browser is away at Google.
	GoogleCookieName   = "__Host-quitsie_google"
	googleFlowLifetime = 10 * time.Minute
)

// google talks to Google's OpenID Connect provider. It discovers the
// provider's endpoints on first use rather than at startup, so the server
// still starts when Google is unreachable.
type google struct {
	cfg GoogleConfig

	mu       sync.Mutex
	oauth    *oauth2.Config
	verifier *oidc.IDTokenVerifier
}

func newGoogle(cfg GoogleConfig) (*google, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" || cfg.RedirectURL == "" {
		return nil, errors.New("auth: Google needs ClientID, ClientSecret and RedirectURL")
	}
	if _, err := url.Parse(cfg.RedirectURL); err != nil {
		return nil, fmt.Errorf("auth: Google RedirectURL: %w", err)
	}
	if cfg.IssuerURL == "" {
		cfg.IssuerURL = "https://accounts.google.com"
	}
	return &google{cfg: cfg}, nil
}

// client returns the OAuth configuration and the ID token verifier,
// discovering the provider the first time.
func (g *google) client(ctx context.Context, now func() time.Time) (*oauth2.Config, *oidc.IDTokenVerifier, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.oauth != nil {
		return g.oauth, g.verifier, nil
	}
	provider, err := oidc.NewProvider(ctx, g.cfg.IssuerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: discover provider: %v", ErrGoogleFailed, err)
	}
	g.oauth = &oauth2.Config{
		ClientID:     g.cfg.ClientID,
		ClientSecret: g.cfg.ClientSecret,
		RedirectURL:  g.cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "email", "profile"},
	}
	g.verifier = provider.Verifier(&oidc.Config{ClientID: g.cfg.ClientID, Now: now})
	return g.oauth, g.verifier, nil
}

// BeginGoogle starts signing in with Google. Set the returned cookie and
// redirect the browser to authURL; Google sends it back to RedirectURL,
// where FinishGoogle takes over and hands returnTo back in SignIn.ReturnTo.
func (s *Service) BeginGoogle(ctx context.Context, returnTo string) (authURL string, flow *http.Cookie, err error) {
	if s.google == nil {
		return "", nil, ErrGoogleDisabled
	}
	oauth, _, err := s.google.client(ctx, s.now)
	if err != nil {
		return "", nil, err
	}

	// state ties Google's answer to this browser (no login CSRF), nonce ties
	// the ID token to this request, and the PKCE verifier makes a stolen
	// authorization code useless.
	state, nonce, verifier := rand.Text(), rand.Text(), oauth2.GenerateVerifier()
	authURL = oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier))
	value := strings.Join([]string{state, nonce, verifier, base64.RawURLEncoding.EncodeToString([]byte(returnTo))}, ".")
	return authURL, hostCookie(GoogleCookieName, value, googleFlowLifetime), nil
}

// FinishGoogle handles the request Google redirects the browser to: it
// checks it against the flow cookie from BeginGoogle, exchanges the code,
// verifies the ID token and signs in the User with that Google account,
// linking or creating it by email if needed. Send ClearGoogleCookie in the
// response whatever the outcome.
func (s *Service) FinishGoogle(ctx context.Context, r *http.Request, device string) (SignIn, error) {
	if s.google == nil {
		return SignIn{}, ErrGoogleDisabled
	}
	q := r.URL.Query()
	switch q.Get("error") {
	case "":
	case "access_denied":
		return SignIn{}, ErrGoogleCancelled
	default:
		return SignIn{}, fmt.Errorf("%w: %s", ErrGoogleFailed, q.Get("error"))
	}

	cookie, err := r.Cookie(GoogleCookieName)
	if err != nil {
		return SignIn{}, fmt.Errorf("%w: no flow cookie (expired, or started in another browser)", ErrGoogleFailed)
	}
	state, nonce, verifier, returnTo, ok := splitFlow(cookie.Value)
	if !ok || subtle.ConstantTimeCompare([]byte(state), []byte(q.Get("state"))) != 1 {
		return SignIn{}, fmt.Errorf("%w: state does not match", ErrGoogleFailed)
	}

	oauth, idVerifier, err := s.google.client(ctx, s.now)
	if err != nil {
		return SignIn{}, err
	}
	token, err := oauth.Exchange(ctx, q.Get("code"), oauth2.VerifierOption(verifier))
	if err != nil {
		return SignIn{}, fmt.Errorf("%w: exchange code: %v", ErrGoogleFailed, err)
	}
	rawIDToken, _ := token.Extra("id_token").(string)
	idToken, err := idVerifier.Verify(ctx, rawIDToken)
	if err != nil {
		return SignIn{}, fmt.Errorf("%w: %v", ErrGoogleFailed, err)
	}
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(nonce)) != 1 {
		return SignIn{}, fmt.Errorf("%w: nonce does not match", ErrGoogleFailed)
	}
	var claims struct {
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return SignIn{}, fmt.Errorf("%w: %v", ErrGoogleFailed, err)
	}
	// An unverified email proves nothing, and linking by it would hand an
	// existing account to whoever typed that address into Google.
	if !claims.EmailVerified {
		return SignIn{}, fmt.Errorf("%w: email not verified by Google", ErrGoogleFailed)
	}
	email, err := NormalizeEmail(claims.Email)
	if err != nil {
		return SignIn{}, fmt.Errorf("%w: %v", ErrGoogleFailed, err)
	}

	var knownDevices string
	if c, err := r.Cookie(KnownDeviceCookieName); err == nil {
		knownDevices = c.Value
	}
	var signIn SignIn
	err = s.inTx(ctx, func(q *store.Queries) error {
		user, created, err := s.userByGoogle(ctx, q, idToken.Subject, email, claims.Name)
		if err != nil {
			return err
		}
		token, err := s.createSession(ctx, q, user.ID, device)
		if err != nil {
			return err
		}
		known, err := s.rememberDevice(ctx, q, user.ID, knownDevices)
		if err != nil {
			return err
		}
		signIn = SignIn{User: user, Token: token, KnownDevices: known, NewUser: created, ReturnTo: returnTo}
		return nil
	})
	if err != nil {
		return SignIn{}, err
	}
	return signIn, nil
}

// ClearGoogleCookie returns a cookie that makes the browser delete the flow
// cookie once FinishGoogle has run.
func ClearGoogleCookie() *http.Cookie {
	c := hostCookie(GoogleCookieName, "", googleFlowLifetime)
	c.MaxAge = -1
	return c
}

func splitFlow(value string) (state, nonce, verifier, returnTo string, ok bool) {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return "", "", "", "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return "", "", "", "", false
	}
	return parts[0], parts[1], parts[2], string(decoded), true
}

// userByGoogle finds the User with this Google account. Failing that, it
// links the account to the User with its (verified) email, or creates one.
func (s *Service) userByGoogle(ctx context.Context, q *store.Queries, sub, email, name string) (user store.User, created bool, err error) {
	user, err = q.GetUserByGoogleSub(ctx, &sub)
	if err == nil {
		return user, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, false, fmt.Errorf("auth: find user by Google account: %w", err)
	}

	user, err = q.GetUserByEmail(ctx, email)
	switch {
	case err == nil:
	case errors.Is(err, sql.ErrNoRows):
		if user, err = s.createUser(ctx, q, email, name); err != nil {
			return store.User{}, false, err
		}
		created = true
	default:
		return store.User{}, false, fmt.Errorf("auth: find user: %w", err)
	}

	if err := q.SetUserGoogleSub(ctx, store.SetUserGoogleSubParams{GoogleSub: &sub, ID: user.ID}); err != nil {
		return store.User{}, false, fmt.Errorf("auth: link Google account: %w", err)
	}
	user.GoogleSub = &sub
	return user, created, nil
}
