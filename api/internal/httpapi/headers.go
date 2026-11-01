package httpapi

import (
	"encoding/json"
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
// headers that make the browser stricter with it. In production it also asks
// for HTTPS only, and sends a visitor that Cloudflare saw on plain HTTP to
// httpsOrigin: Cloudflare's own "Always Use HTTPS" may be off.
func securityHeaders(httpsOrigin string, next http.Handler) http.Handler {
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
		if httpsOrigin == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Without includeSubDomains or preload: those reach the owner's
		// other subdomains and lists outside this server.
		h.Set("Strict-Transport-Security", "max-age=31536000")
		if throughTunnel(r) && visitorUsedHTTP(r) {
			redirectToHTTPS(w, r, httpsOrigin)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// redirectToHTTPS sends r to its path and query on origin. A change gets 308,
// which keeps its method and body where 301 lets browsers turn it into a GET.
func redirectToHTTPS(w http.ResponseWriter, r *http.Request, origin string) {
	target := r.URL.EscapedPath()
	if !strings.HasPrefix(target, "/") {
		target = "/"
	}
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	code := http.StatusPermanentRedirect
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		code = http.StatusMovedPermanently
	}
	// Cloudflare caches the static files, redirects included; this one must
	// never be served to a visitor who is already on HTTPS.
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, origin+target, code)
}

// visitorUsedHTTP reports whether Cloudflare says the visitor asked over
// plain HTTP. Its edge writes the visitor's scheme in both CF-Visitor and
// X-Forwarded-Proto, and cloudflared passes them on. Any https among them
// wins, so a disagreement can never send someone already on HTTPS round in
// a loop. A request with neither, such as the health check from the VM
// itself, is not redirected.
func visitorUsedHTTP(r *http.Request) bool {
	var visitor struct {
		Scheme string `json:"scheme"`
	}
	_ = json.Unmarshal([]byte(r.Header.Get("CF-Visitor")), &visitor)
	schemes := []string{visitor.Scheme}
	for _, v := range r.Header.Values("X-Forwarded-Proto") {
		schemes = append(schemes, strings.Split(v, ",")...)
	}

	plain := false
	for _, s := range schemes {
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "https":
			return false
		case "http":
			plain = true
		}
	}
	return plain
}
