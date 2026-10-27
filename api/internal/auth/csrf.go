package auth

import (
	"net/http"
	"slices"
)

// RequireSameOrigin protects against CSRF: it answers 403 Forbidden to any
// request that may change data (anything but GET, HEAD and OPTIONS) unless
// its Origin header is one of origins, such as "https://quitsie.app".
// Browsers send Origin on every such request and scripts cannot forge it.
// Requests without Origin are refused too, since every legitimate client is
// the PWA running in a browser.
func RequireSameOrigin(origins ...string) func(http.Handler) http.Handler {
	// An empty entry (say, an unset variable) would match a missing header.
	origins = slices.DeleteFunc(slices.Clone(origins), func(o string) bool { return o == "" })
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
			default:
				if !slices.Contains(origins, r.Header.Get("Origin")) {
					http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
