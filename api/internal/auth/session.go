package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// SessionCookieName is the cookie that carries the session token.
const SessionCookieName = "quitsie_session"

const (
	// sessionLifetime is how long a session lasts without being used.
	sessionLifetime = 90 * 24 * time.Hour
	// touchInterval is how stale last_used_at may get before a request
	// renews it, so not every request writes to the database.
	touchInterval = time.Hour
)

// SessionCookie returns the cookie that carries token. The PWA and the API
// share an origin, so the cookie needs no Domain and Lax is enough.
func SessionCookie(token string) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionLifetime / time.Second),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	}
}

// ClearSessionCookie returns a cookie that makes the browser delete the
// session cookie, for signing out.
func ClearSessionCookie() *http.Cookie {
	c := SessionCookie("")
	c.MaxAge = -1
	return c
}

// identity is what Authenticate puts in the request's context.
type identity struct {
	user    store.User
	session Session
}

type identityKey struct{}

// CurrentUser returns the User that Authenticate found for the request.
func CurrentUser(ctx context.Context) (store.User, bool) {
	id, ok := ctx.Value(identityKey{}).(identity)
	return id.user, ok
}

// CurrentSession returns the session that Authenticate found for the request.
func CurrentSession(ctx context.Context) (Session, bool) {
	id, ok := ctx.Value(identityKey{}).(identity)
	return id.session, ok
}

// Authenticate resolves the User from the session cookie, when there is a
// valid one, and makes it available through CurrentUser. It never rejects a
// request for lacking a session; the API decides which operations need one.
// If the session cannot be looked up, fail answers the request.
func (s *Service) Authenticate(fail ErrorHandler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(SessionCookieName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			id, renewed, err := s.resolve(r.Context(), cookie.Value)
			if errors.Is(err, errNoSession) {
				// The session expired or was ended: stop the browser sending it.
				http.SetCookie(w, ClearSessionCookie())
				next.ServeHTTP(w, r)
				return
			}
			if err != nil {
				fail(w, r, err)
				return
			}
			if renewed {
				http.SetCookie(w, SessionCookie(cookie.Value))
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityKey{}, id)))
		})
	}
}

var errNoSession = errors.New("auth: no session")

// resolve finds the session with token and its User, or errNoSession. When
// the session was last used more than touchInterval ago, it moves its last
// use to now, which pushes its expiry 90 days ahead, and reports renewed so
// the cookie gets a new Max-Age too.
func (s *Service) resolve(ctx context.Context, token string) (id identity, renewed bool, err error) {
	q := store.New(s.db)
	row, err := q.GetSessionByTokenHash(ctx, hashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return identity{}, false, errNoSession
	}
	if err != nil {
		return identity{}, false, fmt.Errorf("auth: find session: %w", err)
	}

	now := s.now()
	idle := now.Sub(row.Session.LastUsedAt)
	if idle >= sessionLifetime {
		if _, err := q.DeleteSession(ctx, store.DeleteSessionParams{ID: row.Session.ID, UserID: row.User.ID}); err != nil {
			return identity{}, false, fmt.Errorf("auth: delete expired session: %w", err)
		}
		return identity{}, false, errNoSession
	}
	if idle >= touchInterval {
		if err := q.TouchSession(ctx, store.TouchSessionParams{LastUsedAt: now, ID: row.Session.ID}); err != nil {
			return identity{}, false, fmt.Errorf("auth: touch session: %w", err)
		}
		row.Session.LastUsedAt = now
		renewed = true
	}
	return identity{user: row.User, session: sessionFrom(row.Session)}, renewed, nil
}

// createSession starts a session for the User and returns its token.
func (s *Service) createSession(ctx context.Context, q *store.Queries, userID, device string) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("auth: %w", err)
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	id, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("auth: %w", err)
	}

	now := s.now()
	// Sessions nobody used in 90 days would otherwise stay forever.
	if err := q.DeleteExpiredSessions(ctx, now.Add(-sessionLifetime)); err != nil {
		return "", fmt.Errorf("auth: delete expired sessions: %w", err)
	}
	err = q.CreateSession(ctx, store.CreateSessionParams{
		ID:         id.String(),
		UserID:     userID,
		TokenHash:  hashToken(token),
		Device:     device,
		CreatedAt:  now,
		LastUsedAt: now,
	})
	if err != nil {
		return "", fmt.Errorf("auth: create session: %w", err)
	}
	return token, nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Session is a device where a User is signed in.
type Session struct {
	ID         string
	Device     string
	CreatedAt  time.Time
	LastUsedAt time.Time
}

// Sessions lists the User's live sessions, most recently used first.
func (s *Service) Sessions(ctx context.Context, userID string) ([]Session, error) {
	rows, err := store.New(s.db).ListSessions(ctx, store.ListSessionsParams{
		UserID:    userID,
		UsedAfter: s.now().Add(-sessionLifetime),
	})
	if err != nil {
		return nil, fmt.Errorf("auth: list sessions: %w", err)
	}
	sessions := make([]Session, len(rows))
	for i, row := range rows {
		sessions[i] = sessionFrom(row)
	}
	return sessions, nil
}

func sessionFrom(row store.Session) Session {
	return Session{ID: row.ID, Device: row.Device, CreatedAt: row.CreatedAt, LastUsedAt: row.LastUsedAt}
}

// ErrSessionNotFound means the User has no session with that ID.
var ErrSessionNotFound = errors.New("auth: session not found")

// EndSession signs the User out of one device. To sign out of the current
// one, pass CurrentSession's ID and send ClearSessionCookie.
func (s *Service) EndSession(ctx context.Context, userID, sessionID string) error {
	n, err := store.New(s.db).DeleteSession(ctx, store.DeleteSessionParams{ID: sessionID, UserID: userID})
	if err != nil {
		return fmt.Errorf("auth: end session: %w", err)
	}
	if n == 0 {
		return ErrSessionNotFound
	}
	return nil
}
