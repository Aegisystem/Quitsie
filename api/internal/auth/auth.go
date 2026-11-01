package auth

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/mail"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Config holds what the Service needs. DB, Mail and DefaultCurrency are
// required; the rest is optional.
type Config struct {
	DB   *sql.DB
	Mail mail.Sender
	// DefaultCurrency is the PreferredCurrency of new Users until they change it.
	DefaultCurrency string
	// Google enables signing in with Google; nil disables it.
	Google *GoogleConfig
	// MailPerHour and MailPerDay are the most login-code emails the server
	// sends in a clock hour and in 24 hours. Zero means DefaultMailPerHour
	// and DefaultMailPerDay.
	MailPerHour, MailPerDay int
	// Logger defaults to slog.Default; it says when the budget of emails is
	// spent.
	Logger *slog.Logger
	// Now defaults to time.Now; tests replace it to move the clock.
	Now func() time.Time
}

// Service signs Users in and keeps track of their sessions.
type Service struct {
	db              *sql.DB
	mail            mail.Sender
	defaultCurrency string
	now             func() time.Time
	logger          *slog.Logger
	mailPerHour     int
	mailPerDay      int
	sends           *limiter
	verifications   *limiter
	google          *google
}

func New(cfg Config) (*Service, error) {
	if cfg.DB == nil || cfg.Mail == nil || cfg.DefaultCurrency == "" {
		return nil, errors.New("auth: DB, Mail and DefaultCurrency are required")
	}
	if cfg.MailPerHour < 0 || cfg.MailPerDay < 0 {
		return nil, errors.New("auth: MailPerHour and MailPerDay cannot be negative")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	s := &Service{
		db:              cfg.DB,
		mail:            cfg.Mail,
		defaultCurrency: cfg.DefaultCurrency,
		now:             func() time.Time { return cfg.Now().UTC() },
		logger:          cfg.Logger,
		mailPerHour:     cmp.Or(cfg.MailPerHour, DefaultMailPerHour),
		mailPerDay:      cmp.Or(cfg.MailPerDay, DefaultMailPerDay),
		sends:           newLimiter(sendWindow),
		verifications:   newLimiter(sendWindow),
	}
	if cfg.Google != nil {
		g, err := newGoogle(*cfg.Google)
		if err != nil {
			return nil, err
		}
		s.google = g
	}
	return s, nil
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
	// ReturnTo is what BeginGoogle was given; it comes from a cookie, so
	// check it before redirecting there. Empty for a sign-in with a code.
	ReturnTo string
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
	// Signing in by code tells us nothing but the email, so the name stays
	// empty until the PWA asks for it (see SignIn.NewUser).
	user, err = s.createUser(ctx, q, email, "")
	if err != nil {
		return store.User{}, false, err
	}
	return user, true, nil
}

// createUser creates a User with the default PreferredCurrency.
func (s *Service) createUser(ctx context.Context, q *store.Queries, email, name string) (store.User, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return store.User{}, fmt.Errorf("auth: %w", err)
	}
	user := store.User{
		ID:                id.String(),
		Email:             email,
		Name:              name,
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
		return store.User{}, fmt.Errorf("auth: create user: %w", err)
	}
	return user, nil
}
