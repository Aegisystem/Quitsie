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
	// HTTPSOrigin is the public origin in production, such as
	// "https://quitsie.app", and empty in development. With it every
	// response asks for HTTPS only (HSTS), and a visitor Cloudflare saw on
	// plain HTTP is sent there.
	HTTPSOrigin string
}

// WithDeps gives the handlers their dependencies.
func (s *Server) WithDeps(d Deps) *Server {
	s.deps = d
	return s
}
