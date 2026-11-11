package httpapi

import (
	"database/sql"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
	"github.com/Aegisystem/Quitsie/api/internal/changefeed"
)

// Deps are what the handlers use besides the health check's Pinger. Each area
// of the API (auth, ledgers, records) adds the fields it needs here, and
// cmd/server wires them.
type Deps struct {
	DB *sql.DB
	// Changes says when a ledger changed, for the open devices and the push
	// notifications (ADR 0013). Nil in tests that do not need it.
	Changes *changefeed.Feed
	// Auth signs Users in and resolves the session of every request. Only a
	// test that never reaches the sign-in operations can leave it out; then
	// nobody is signed in.
	Auth *auth.Service
	// Origins are the app's own origins, such as "https://quitsie.app".
	// Requests that change data must come from one of them (CSRF).
	Origins []string
	// InviteSecret signs the Invite links (ADR 0010): at least
	// invites.MinSecretLength random bytes. Changing it revokes every link.
	InviteSecret []byte
	// PushKey is the VAPID public key the devices subscribe to push
	// notifications with (ADR 0013); empty when the server sends none.
	PushKey string
	// Now is the clock of the limit of requests per IP; nil is time.Now.
	Now func() time.Time
	// CloudflareSecret is what Cloudflare writes on every request it forwards,
	// in the header X-Quitsie-Cloudflare-Secret (ADR 0014). A request that
	// carries it came through Cloudflare, so its CF-Connecting-IP is the
	// address of the visitor, and the limits per IP count it as that one.
	// Every other request counts as one same client. Empty when nothing adds
	// the secret, as in development: then no header is believed.
	CloudflareSecret string
	// HSTS makes every response ask for HTTPS only. Production sets it;
	// it would lock a developer's localhost out of plain HTTP.
	HSTS bool
}

// WithDeps gives the handlers their dependencies.
func (s *Server) WithDeps(d Deps) *Server {
	s.deps = d
	return s
}
