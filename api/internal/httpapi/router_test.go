package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/Aegisystem/Quitsie/api/internal/httpapi"
)

type fakeDB struct{ err error }

func (f fakeDB) PingContext(context.Context) error { return f.err }

var builtPWA = fstest.MapFS{
	"index.html":            {Data: []byte("<!doctype html><title>Quitsie</title>")},
	"sw.js":                 {Data: []byte("self.addEventListener('install', () => {})")},
	"assets/index-a1b2c.js": {Data: []byte("console.log('quitsie')")},
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestHealth(t *testing.T) {
	tests := []struct {
		name     string
		dbErr    error
		code     int
		status   httpapi.HealthStatus
		database httpapi.HealthDatabase
	}{
		{"database up", nil, http.StatusOK, httpapi.HealthStatusOk, httpapi.HealthDatabaseOk},
		{"database down", errors.New("disk gone"), http.StatusServiceUnavailable, httpapi.HealthStatusUnavailable, httpapi.HealthDatabaseUnreachable},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := httpapi.NewHandler(httpapi.NewServer(fakeDB{tt.dbErr}, "test"), builtPWA)

			rec := get(t, h, "/api/v1/health")

			if rec.Code != tt.code {
				t.Fatalf("code = %d, want %d", rec.Code, tt.code)
			}
			var body httpapi.Health
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			want := httpapi.Health{Status: tt.status, Database: tt.database, Version: "test"}
			if body != want {
				t.Errorf("body = %+v, want %+v", body, want)
			}
		})
	}
}

func TestRouting(t *testing.T) {
	h := httpapi.NewHandler(httpapi.NewServer(fakeDB{}, "test"), builtPWA)

	tests := []struct {
		name         string
		target       string
		code         int
		cacheControl string
	}{
		{"root serves index.html", "/", http.StatusOK, "no-cache"},
		{"client route falls back to index.html", "/groups/019a2b3c", http.StatusOK, "no-cache"},
		{"service worker is revalidated", "/sw.js", http.StatusOK, "no-cache"},
		{"hashed asset is immutable", "/assets/index-a1b2c.js", http.StatusOK, "public, max-age=31536000, immutable"},
		{"missing asset is not index.html", "/assets/missing.js", http.StatusNotFound, ""},
		{"unknown API route is not index.html", "/api/v1/nope", http.StatusNotFound, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := get(t, h, tt.target)

			if rec.Code != tt.code {
				t.Errorf("code = %d, want %d", rec.Code, tt.code)
			}
			if got := rec.Header().Get("Cache-Control"); got != tt.cacheControl {
				t.Errorf("Cache-Control = %q, want %q", got, tt.cacheControl)
			}
		})
	}
}

func TestPWANotBuilt(t *testing.T) {
	h := httpapi.NewHandler(httpapi.NewServer(fakeDB{}, "test"), fstest.MapFS{})

	if rec := get(t, h, "/"); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if rec := get(t, h, "/api/v1/health"); rec.Code != http.StatusOK {
		t.Errorf("health code = %d, want %d even without the PWA", rec.Code, http.StatusOK)
	}
}
