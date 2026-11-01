package auth

import (
	"net/http"
	"time"
)

// hostCookie returns a cookie of sign-in that lasts maxAge. clearHostCookie
// deletes it.
//
// Every cookie of sign-in has a name with the __Host- prefix, so the browser
// only takes it from this host over HTTPS, with Path=/ and without Domain.
// Without the prefix, a page on any other subdomain of the app's domain, such
// as a forgotten DNS record, could set a cookie of the same name for the whole
// domain with a longer Path, and the browser would send it first. The server
// never accepts a token the browser chose, but a planted cookie still does
// harm: a session or a Google flow of the planter's signs the browser in to
// the planter's account, and a known-device or code flow cookie stands in
// front of the real one, which locks its User out again (ADR 0011) or fails
// the code. __Secure- with narrower paths would not stop it, because that
// subdomain is on HTTPS too. The price is Path=/: the flow and known-device
// cookies, which only sign-in reads, travel with every request, a few hundred
// bytes at most.
func hostCookie(name, value string, maxAge time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   int(maxAge / time.Second),
		HttpOnly: true,
		Secure:   true,
		// The PWA and the API share an origin, so Lax is enough, and it still
		// sends them on the top-level redirect back from Google.
		SameSite: http.SameSiteLaxMode,
	}
}

// clearHostCookie returns a cookie that makes the browser delete the cookie of
// sign-in with this name. It has the attributes of hostCookie's, because the
// browser takes the deletion only with the same ones.
func clearHostCookie(name string) *http.Cookie {
	c := hostCookie(name, "", 0)
	c.MaxAge = -1
	return c
}

// cookieValue returns the value of r's cookie with this name, empty without
// one.
func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
