package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

// errNotImplemented is returned by the handlers of operations that are not
// built yet; it becomes 501 not_implemented.
var errNotImplemented = errors.New("not implemented")

// writeProblem writes an application/problem+json body (RFC 9457).
func writeProblem(w http.ResponseWriter, status int, code ProblemCode, title, detail string) {
	p := Problem{Status: status, Code: code, Title: title}
	if detail != "" {
		p.Detail = &detail
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

// badRequest answers requests whose parameters or body do not match openapi.yaml.
func badRequest(w http.ResponseWriter, _ *http.Request, err error) {
	writeProblem(w, http.StatusBadRequest, ProblemCodeValidationFailed, "The request does not match the API contract.", err.Error())
}

// handlerError answers the errors a handler returns instead of a response.
// Their text may hold internal details, so it goes to the log, not the client.
func handlerError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, errNotImplemented) {
		writeProblem(w, http.StatusNotImplemented, ProblemCodeNotImplemented, "This operation is not implemented yet.", "")
		return
	}
	slog.ErrorContext(r.Context(), "handler failed", "method", r.Method, "path", r.URL.Path, "err", err)
	writeProblem(w, http.StatusInternalServerError, ProblemCodeInternalError, "Internal server error.", "")
}
