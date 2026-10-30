package auth

import (
	"errors"
	"net/http"
	"slices"
)

// ErrForeignOrigin means a request that may change data did not come from
// one of the app's own origins.
var ErrForeignOrigin = errors.New("auth: request from a foreign origin")

// ErrorHandler answers the requests a middleware of this package refuses or
// cannot handle, in the API's own error format. err is ErrForeignOrigin or
// an internal error.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// RequireSameOrigin protects against CSRF: it refuses with ErrForeignOrigin
// any request that may change data (anything but GET, HEAD and OPTIONS)
// unless its Origin header is one of origins, such as "https://quitsie.app".
// Browsers send Origin on every such request and scripts cannot forge it.
// Requests without Origin are refused too, since every legitimate client is
// the PWA running in a browser.
func RequireSameOrigin(refuse ErrorHandler, origins ...string) func(http.Handler) http.Handler {
	// An empty entry (say, an unset variable) would match a missing header.
	origins = slices.DeleteFunc(slices.Clone(origins), func(o string) bool { return o == "" })
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
			default:
				if !slices.Contains(origins, r.Header.Get("Origin")) {
					refuse(w, r, ErrForeignOrigin)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
