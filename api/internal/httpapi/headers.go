package httpapi

import (
	"net/http"
	"strings"
)

// contentSecurityPolicy lets a page run and load only the files of the PWA,
// so that a future XSS could neither bring in code nor send the replica
// anywhere else. The built app has no inline scripts or styles: React sets
// style={{}} through the CSSOM, which this does not block. The data: images
// are SVG masks in the stylesheet. Google sign-in is a navigation, not a
// form, so form-action does not stand in its way.
const contentSecurityPolicy = "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; manifest-src 'self'; worker-src 'self'; " +
	"base-uri 'none'; frame-ancestors 'none'; form-action 'self'"

// permissionsPolicy turns off what the app never uses. It leaves out the
// clipboard and Web Share, which copy and share the Invite links. Chrome
// warns in the console about any feature it does not know, so the list only
// has long-standing ones.
const permissionsPolicy = "accelerometer=(), camera=(), display-capture=(), geolocation=(), " +
	"gyroscope=(), hid=(), magnetometer=(), microphone=(), midi=(), payment=(), serial=(), usb=()"

// securityHeaders puts on every response of the binary, PWA and API, the
// headers that make the browser stricter with it. With hsts, in production, it
// also asks for HTTPS only.
//
// The server redirects nobody to HTTPS: it cannot tell who came over plain
// HTTP, because Railway's proxy writes X-Forwarded-Proto: https on every
// request. Cloudflare does it with "Always Use HTTPS", and Railway for whoever
// reaches it directly (docs/despliegue.md).
func securityHeaders(hsts bool, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", contentSecurityPolicy)
		h.Set("X-Content-Type-Options", "nosniff")
		// same-origin rather than no-referrer: with no-referrer, browsers
		// send "Origin: null" on a form POST, and the CSRF check reads Origin.
		h.Set("Referrer-Policy", "same-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Permissions-Policy", permissionsPolicy)
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			// API answers are about one User; no cache keeps them, not even
			// after that User signs out. The PWA's files keep their own rules.
			h.Set("Cache-Control", "no-store")
		}
		if hsts {
			// Without includeSubDomains or preload: those reach the owner's
			// other subdomains and lists outside this server.
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		next.ServeHTTP(w, r)
	})
}
