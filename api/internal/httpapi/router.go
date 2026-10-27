package httpapi

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewHandler serves the whole binary from one origin: the API under /api/v1
// and the compiled PWA everywhere else, so there is no CORS to configure.
func NewHandler(s *Server, pwa fs.FS) http.Handler {
	mux := http.NewServeMux()
	HandlerWithOptions(NewStrictHandler(s, nil), StdHTTPServerOptions{
		BaseURL:    "/api/v1",
		BaseRouter: mux,
	})
	// Unknown API paths must answer 404 instead of falling through to the PWA.
	mux.Handle("/api/", http.NotFoundHandler())
	mux.Handle("/", pwaHandler(pwa))
	return mux
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
