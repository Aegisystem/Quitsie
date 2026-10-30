package auth_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
)

// refuse answers 403 to requests RequireSameOrigin refuses with ErrForeignOrigin.
func refuse(w http.ResponseWriter, _ *http.Request, err error) {
	if errors.Is(err, auth.ErrForeignOrigin) {
		w.WriteHeader(http.StatusForbidden)
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}

// send runs a request with the given Origin header (none if empty) through
// RequireSameOrigin for https://quitsie.app and returns the status code.
func send(method, origin string) int {
	h := auth.RequireSameOrigin(refuse, "https://quitsie.app")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(method, "https://quitsie.app/api/v1/expenses", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestChangesMustComeFromOurOwnOrigin(t *testing.T) {
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		if code := send(method, "https://quitsie.app"); code != http.StatusOK {
			t.Errorf("%s from https://quitsie.app: status = %d, want 200", method, code)
		}
		for _, origin := range []string{"", "null", "https://evil.example", "http://quitsie.app", "https://quitsie.app.evil.example"} {
			if code := send(method, origin); code != http.StatusForbidden {
				t.Errorf("%s from %q: status = %d, want 403", method, origin, code)
			}
		}
	}
}

func TestReadsNeedNoOrigin(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		if code := send(method, ""); code != http.StatusOK {
			t.Errorf("%s without Origin: status = %d, want 200", method, code)
		}
	}
}

// An unset origin in the configuration must not let requests without Origin through.
func TestAnEmptyAllowedOriginAllowsNothing(t *testing.T) {
	h := auth.RequireSameOrigin(refuse, "")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/expenses", nil))

	if rec.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", rec.Code)
	}
}
