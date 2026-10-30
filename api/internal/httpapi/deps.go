package httpapi

import (
	"database/sql"

	"github.com/Aegisystem/Quitsie/api/internal/auth"
)

// Deps are what the handlers use besides the health check's Pinger. Each area
// of the API (auth, ledgers, records) adds the fields it needs here, and
// cmd/server wires them.
type Deps struct {
	DB *sql.DB
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
}

// WithDeps gives the handlers their dependencies.
func (s *Server) WithDeps(d Deps) *Server {
	s.deps = d
	return s
}
