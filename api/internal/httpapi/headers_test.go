package httpapi_test

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
)

// The headers every response carries, PWA and API alike. They are spelled out
// so that weakening one is a change someone has to make on purpose.
var securityHeaders = map[string]string{
	"Content-Security-Policy": "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
		"font-src 'self'; connect-src 'self'; manifest-src 'self'; worker-src 'self'; " +
		"base-uri 'none'; frame-ancestors 'none'; form-action 'self'",
	"X-Content-Type-Options":     "nosniff",
	"Referrer-Policy":            "same-origin",
	"Cross-Origin-Opener-Policy": "same-origin",
	"Permissions-Policy": "accelerometer=(), camera=(), display-capture=(), geolocation=(), " +
		"gyroscope=(), hid=(), magnetometer=(), microphone=(), midi=(), payment=(), serial=(), usb=()",
}

const hsts = "max-age=31536000"

// served answers with the handler of production, or with the one of
// development.
func served(production bool) http.Handler {
	return httpapi.NewHandler(httpapi.NewServer(fakeDB{}, "test").WithDeps(httpapi.Deps{HSTS: production}), builtPWA)
}

// ask sends method target over a connection from peer, with headers.
func ask(h http.Handler, method, target, peer string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	req.RemoteAddr = peer
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// Answers of every kind: the PWA, its files, the API, and the API's errors,
// including the ones middlewares write before any handler runs.
var everyKindOfAnswer = []struct {
	name         string
	method       string
	target       string
	code         int
	cacheControl string
}{
	{"the PWA", http.MethodGet, "/", http.StatusOK, "no-cache"},
	{"a client route", http.MethodGet, "/groups/019a2b3c", http.StatusOK, "no-cache"},
	{"the service worker", http.MethodGet, "/sw.js", http.StatusOK, "no-cache"},
	{"a hashed asset", http.MethodGet, "/assets/index-a1b2c.js", http.StatusOK, "public, max-age=31536000, immutable"},
	{"a missing asset", http.MethodGet, "/assets/missing.js", http.StatusNotFound, ""},
	{"the health check", http.MethodGet, "/api/v1/health", http.StatusOK, "no-store"},
	{"an operation without a session", http.MethodGet, "/api/v1/me", http.StatusUnauthorized, "no-store"},
	{"a change from another origin", http.MethodPost, "/api/v1/groups", http.StatusForbidden, "no-store"},
	{"an unknown API path", http.MethodGet, "/api/v1/nope", http.StatusNotFound, "no-store"},
	{"the API root without its slash", http.MethodGet, "/api", http.StatusTemporaryRedirect, "no-store"},
}

func TestEveryAnswerCarriesTheSecurityHeaders(t *testing.T) {
	h := served(false)
	for _, tt := range everyKindOfAnswer {
		t.Run(tt.name, func(t *testing.T) {
			rec := ask(h, tt.method, tt.target, direct, nil)

			if rec.Code != tt.code {
				t.Fatalf("code = %d, want %d", rec.Code, tt.code)
			}
			for name, want := range securityHeaders {
				if got := rec.Header().Get(name); got != want {
					t.Errorf("%s = %q, want %q", name, got, want)
				}
			}
		})
	}
}

// The browser must never keep an API answer, which belongs to one User, while
// the PWA's files keep the caching that lets them load fast and offline.
func TestOnlyAPIAnswersAreNeverStored(t *testing.T) {
	h := served(false)
	for _, tt := range everyKindOfAnswer {
		t.Run(tt.name, func(t *testing.T) {
			rec := ask(h, tt.method, tt.target, direct, nil)

			if got := rec.Header().Get("Cache-Control"); got != tt.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cacheControl)
			}
		})
	}
}

// The limit per IP answers before anything else, and still with the headers.
func TestARefusedRequestCarriesTheSecurityHeaders(t *testing.T) {
	h, _ := limited(t)
	spend(t, h, direct, "")

	for _, target := range []string{"/", "/api/v1/me"} {
		rec := visit(h, direct, "", target)

		if rec.Code != http.StatusTooManyRequests {
			t.Fatalf("%s: code = %d, want 429", target, rec.Code)
		}
		if got := rec.Header().Get("Content-Security-Policy"); got != securityHeaders["Content-Security-Policy"] {
			t.Errorf("%s: Content-Security-Policy = %q", target, got)
		}
	}
	if got := visit(h, direct, "", "/api/v1/me").Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("API 429: Cache-Control = %q, want no-store", got)
	}
}

// Production keeps every other header and adds HSTS.
func TestOnlyProductionAsksForHTTPSOnly(t *testing.T) {
	production, development := served(true), served(false)
	for _, tt := range everyKindOfAnswer {
		t.Run(tt.name, func(t *testing.T) {
			rec := ask(production, tt.method, tt.target, direct, nil)

			if rec.Code != tt.code {
				t.Fatalf("code = %d, want %d", rec.Code, tt.code)
			}
			want := maps.Clone(securityHeaders)
			want["Strict-Transport-Security"] = hsts
			for name, want := range want {
				if got := rec.Header().Get(name); got != want {
					t.Errorf("production: %s = %q, want %q", name, got, want)
				}
			}
			if got := ask(development, tt.method, tt.target, direct, nil).Header().Get("Strict-Transport-Security"); got != "" {
				t.Errorf("development: Strict-Transport-Security = %q, want none", got)
			}
		})
	}
}

// Cloudflare sends a visitor on plain HTTP to HTTPS ("Always Use HTTPS"), and
// Railway does the same with whoever reaches it directly. The server cannot:
// Railway writes X-Forwarded-Proto: https on every request, whatever the
// visitor used. A redirect of its own, on what a header says, could only send
// someone already on HTTPS round in a loop.
func TestTheServerLeavesTheRedirectToHTTPSToTheProxies(t *testing.T) {
	saysHTTP := map[string]string{"X-Forwarded-Proto": "http", "CF-Visitor": `{"scheme":"http"}`}
	h := served(true)
	for _, peer := range []string{direct, "127.0.0.1:40000"} {
		for _, method := range []string{http.MethodGet, http.MethodPost} {
			rec := ask(h, method, "/api/v1/health", peer, saysHTTP)

			if got := rec.Header().Get("Location"); got != "" || rec.Code/100 == 3 {
				t.Errorf("%s from %s: code = %d (Location %q), want no redirect", method, peer, rec.Code, got)
			}
			if got := rec.Header().Get("Strict-Transport-Security"); got != hsts {
				t.Errorf("%s from %s: Strict-Transport-Security = %q, want %q", method, peer, got, hsts)
			}
		}
	}
}
