package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/mail"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Config holds what the Service needs. Everything but Now is required.
type Config struct {
	DB   *sql.DB
	Mail mail.Sender
	// DefaultCurrency is the PreferredCurrency of new Users until they change it.
	DefaultCurrency string
	// Now defaults to time.Now; tests replace it to move the clock.
	Now func() time.Time
}

// Service signs Users in and keeps track of their sessions.
type Service struct {
	db              *sql.DB
	mail            mail.Sender
	defaultCurrency string
	now             func() time.Time
}

func New(cfg Config) (*Service, error) {
	if cfg.DB == nil || cfg.Mail == nil || cfg.DefaultCurrency == "" {
		return nil, errors.New("auth: DB, Mail and DefaultCurrency are required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Service{
		db:              cfg.DB,
		mail:            cfg.Mail,
		defaultCurrency: cfg.DefaultCurrency,
		now:             func() time.Time { return cfg.Now().UTC() },
	}, nil
}

// SignIn is the outcome of a successful sign-in.
type SignIn struct {
	User store.User
	// Token goes in the session cookie (see SessionCookie). This is the only
	// copy: the database keeps just its hash.
	Token string
	// NewUser is true when this sign-in created the User, so the PWA can ask
	// for a name and a PreferredCurrency.
	NewUser bool
}

// inTx runs fn in a write transaction. The store opens SQLite with
// _txlock=immediate, so transactions take the write lock when they begin
// and concurrent sign-ins run one after the other.
func (s *Service) inTx(ctx context.Context, fn func(q *store.Queries) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	defer tx.Rollback()
	if err := fn(store.New(tx)); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	return nil
}

// userByEmail finds the User with this email or creates one.
func (s *Service) userByEmail(ctx context.Context, q *store.Queries, email string) (user store.User, created bool, err error) {
	user, err = q.GetUserByEmail(ctx, email)
	if err == nil {
		return user, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return store.User{}, false, fmt.Errorf("auth: find user: %w", err)
	}

	id, err := uuid.NewV7()
	if err != nil {
		return store.User{}, false, fmt.Errorf("auth: %w", err)
	}
	user = store.User{
		ID:                id.String(),
		Email:             email,
		PreferredCurrency: s.defaultCurrency,
		CreatedAt:         s.now(),
	}
	err = q.CreateUser(ctx, store.CreateUserParams{
		ID:                user.ID,
		Email:             user.Email,
		Name:              user.Name,
		PreferredCurrency: user.PreferredCurrency,
		CreatedAt:         user.CreatedAt,
	})
	if err != nil {
		return store.User{}, false, fmt.Errorf("auth: create user: %w", err)
	}
	return user, true, nil
}
