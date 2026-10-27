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
	// Auth signs Users in and resolves the session of every request. Without
	// it nobody is signed in, so only the public operations answer.
	Auth *auth.Service
	// Origins are the app's own origins, such as "https://quitsie.app".
	// Requests that change data must come from one of them (CSRF).
	Origins []string
}

// WithDeps gives the handlers their dependencies.
func (s *Server) WithDeps(d Deps) *Server {
	s.deps = d
	return s
}
