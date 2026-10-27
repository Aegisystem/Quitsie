package httpapi

import "database/sql"

// Deps are what the handlers use besides the health check's Pinger. Each area
// of the API (auth, ledgers, records) adds the fields it needs here, and
// cmd/server wires them.
type Deps struct {
	DB *sql.DB
}

// WithDeps gives the handlers their dependencies.
func (s *Server) WithDeps(d Deps) *Server {
	s.deps = d
	return s
}
