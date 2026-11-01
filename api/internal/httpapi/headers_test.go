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

const (
	productionOrigin = "https://quitsie.test"
	hsts             = "max-age=31536000"
)

// served answers with the handler of production when httpsOrigin is set,
// and with the one of development when it is empty.
func served(httpsOrigin string) http.Handler {
	return httpapi.NewHandler(httpapi.NewServer(fakeDB{}, "test").WithDeps(httpapi.Deps{HTTPSOrigin: httpsOrigin}), builtPWA)
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
	h := served("")
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
	h := served("")
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
	production, development := served(productionOrigin), served("")
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

var (
	visitorOnHTTP  = map[string]string{"X-Forwarded-Proto": "http", "CF-Visitor": `{"scheme":"http"}`}
	visitorOnHTTPS = map[string]string{"X-Forwarded-Proto": "https", "CF-Visitor": `{"scheme":"https"}`}
)

// Cloudflare may serve plain HTTP if its "Always Use HTTPS" is off; then the
// server itself sends the visitor to HTTPS, with the same path and query.
func TestAVisitorOnPlainHTTPIsSentToHTTPS(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		target   string
		peer     string
		headers  map[string]string
		code     int
		location string
	}{
		{"a page", http.MethodGet, "/groups/019a?tab=a%2Fb", tunnel, visitorOnHTTP,
			http.StatusMovedPermanently, productionOrigin + "/groups/019a?tab=a%2Fb"},
		{"a HEAD", http.MethodHead, "/sw.js", tunnel, visitorOnHTTP,
			http.StatusMovedPermanently, productionOrigin + "/sw.js"},
		// 308 keeps the method and the body, where 301 may turn them into a GET.
		{"a change", http.MethodPost, "/api/v1/auth/login-code", tunnel, visitorOnHTTP,
			http.StatusPermanentRedirect, productionOrigin + "/api/v1/auth/login-code"},
		{"only X-Forwarded-Proto says so", http.MethodGet, "/", tunnel, map[string]string{"X-Forwarded-Proto": "http"},
			http.StatusMovedPermanently, productionOrigin + "/"},
		{"only CF-Visitor says so", http.MethodGet, "/", tunnel, map[string]string{"CF-Visitor": `{"scheme":"http"}`},
			http.StatusMovedPermanently, productionOrigin + "/"},
		{"cloudflared over IPv6", http.MethodGet, "/", "[::1]:40000", visitorOnHTTP,
			http.StatusMovedPermanently, productionOrigin + "/"},
		{"a path that looks like another host", http.MethodGet, "//evil.example/x", tunnel, visitorOnHTTP,
			http.StatusMovedPermanently, productionOrigin + "//evil.example/x"},
	}
	h := served(productionOrigin)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := ask(h, tt.method, tt.target, tt.peer, tt.headers)

			if rec.Code != tt.code {
				t.Fatalf("code = %d, want %d", rec.Code, tt.code)
			}
			if got := rec.Header().Get("Location"); got != tt.location {
				t.Errorf("Location = %q, want %q", got, tt.location)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store so that no cache serves it over HTTPS", got)
			}
		})
	}
}

func TestOnlyAVisitorCloudflareSawOnHTTPIsRedirected(t *testing.T) {
	disagree := maps.Clone(visitorOnHTTP)
	disagree["CF-Visitor"] = `{"scheme":"https"}`
	tests := []struct {
		name    string
		origin  string
		peer    string
		headers map[string]string
	}{
		// The deploy script checks the new binary straight on 127.0.0.1.
		{"the health check from the VM", productionOrigin, tunnel, nil},
		{"a visitor on HTTPS", productionOrigin, tunnel, visitorOnHTTPS},
		// A redirect then could send that visitor round in a loop.
		{"headers that disagree", productionOrigin, tunnel, disagree},
		{"a CF-Visitor that is not JSON", productionOrigin, tunnel, map[string]string{"CF-Visitor": "http"}},
		// Only cloudflared, on loopback, can vouch for those headers.
		{"a connection from outside the tunnel", productionOrigin, direct, visitorOnHTTP},
		{"development", "", tunnel, visitorOnHTTP},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, target := range []string{"/", "/api/v1/health"} {
				rec := ask(served(tt.origin), http.MethodGet, target, tt.peer, tt.headers)

				if rec.Code != http.StatusOK {
					t.Errorf("%s: code = %d (Location %q), want 200", target, rec.Code, rec.Header().Get("Location"))
				}
			}
		})
	}
}
