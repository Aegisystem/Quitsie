package httpapi

import (
	"context"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
)

// NewHandler serves the whole binary from one origin: the API under /api/v1
// and the compiled PWA everywhere else, so there is no CORS to configure.
func NewHandler(s *Server, pwa fs.FS) http.Handler {
	api := http.NewServeMux()
	ops := NewStrictHandlerWithOptions(s, []StrictMiddlewareFunc{requireSession, withRequest}, StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequest,
		ResponseErrorHandlerFunc: handlerError,
	})
	HandlerWithOptions(ops, StdHTTPServerOptions{
		BaseURL:          "/api/v1",
		BaseRouter:       api,
		ErrorHandlerFunc: badRequest,
	})
	// Unknown API paths must answer 404 instead of falling through to the PWA.
	api.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeProblem(w, http.StatusNotFound, ProblemCodeNotFound, "There is no such API endpoint.", "")
	})

	mux := http.NewServeMux()
	mux.Handle("/api/", s.authenticate(api))
	mux.Handle("/", pwaHandler(pwa))
	return mux
}

// authenticate finds the User of every API request from its session cookie.
func (s *Server) authenticate(next http.Handler) http.Handler {
	if s.deps.Auth == nil {
		return next
	}
	return s.deps.Auth.Authenticate(next)
}

// publicOperations answer without a session; every other operation needs
// one. openapi.yaml says the same with `security: []`, but the generated
// code does not apply it. GetInvite shows an invitation to someone who may
// not have signed in yet.
var publicOperations = map[string]bool{
	"GetHealth":         true,
	"RequestLoginCode":  true,
	"VerifyLoginCode":   true,
	"StartGoogleLogin":  true,
	"FinishGoogleLogin": true,
	"GetInvite":         true,
}

// requireSession answers 401 unauthenticated to any operation outside
// publicOperations when authenticate found no User, so handlers of those
// operations can count on auth.CurrentUser.
func requireSession(f StrictHandlerFunc, operationID string) StrictHandlerFunc {
	if publicOperations[operationID] {
		return f
	}
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		if _, ok := auth.CurrentUser(ctx); !ok {
			return nil, errUnauthenticated
		}
		return f(ctx, w, r, request)
	}
}

type requestKey struct{}

// withRequest makes the HTTP request available to the strict handlers,
// which only get its decoded parameters, through requestFrom.
func withRequest(f StrictHandlerFunc, _ string) StrictHandlerFunc {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request, request any) (any, error) {
		return f(context.WithValue(ctx, requestKey{}, r), w, r, request)
	}
}

// requestFrom returns the HTTP request a strict handler is answering, for
// what its parameters do not carry: the client's address, its User-Agent
// and cookies.
func requestFrom(ctx context.Context) *http.Request {
	return ctx.Value(requestKey{}).(*http.Request)
}

// pwaHandler serves the files of the compiled PWA. Paths without an
// extension are client-side routes and get index.html.
func pwaHandler(pwa fs.FS) http.Handler {
	files := http.FileServerFS(pwa)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if info, err := fs.Stat(pwa, name); name == "" || err != nil || info.IsDir() {
			if name != "" && path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			if _, err := fs.Stat(pwa, "index.html"); err != nil {
				http.Error(w, "The PWA is not built: run `npm run build` in web/.", http.StatusServiceUnavailable)
				return
			}
			name = "index.html"
			r = r.Clone(r.Context())
			r.URL.Path = "/"
		}

		if strings.HasPrefix(name, "assets/") {
			// Vite puts a content hash in these file names, so they never change.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// index.html, sw.js and the manifest must be revalidated so updates reach users.
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
