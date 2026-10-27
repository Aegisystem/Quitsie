package httpapi

import (
	"context"
	"time"
)

// Pinger is the part of *sql.DB the health check needs.
type Pinger interface {
	PingContext(ctx context.Context) error
}

// Server implements the handlers declared in openapi.yaml.
type Server struct {
	db      Pinger
	version string
}

var _ StrictServerInterface = (*Server)(nil)

func NewServer(db Pinger, version string) *Server {
	return &Server{db: db, version: version}
}

func (s *Server) GetHealth(ctx context.Context, _ GetHealthRequestObject) (GetHealthResponseObject, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		return GetHealth503JSONResponse{
			Status:   HealthStatusUnavailable,
			Database: HealthDatabaseUnreachable,
			Version:  s.version,
		}, nil
	}
	return GetHealth200JSONResponse{
		Status:   HealthStatusOk,
		Database: HealthDatabaseOk,
		Version:  s.version,
	}, nil
}
