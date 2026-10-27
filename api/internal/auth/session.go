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

type userKey struct{}

// CurrentUser returns the User that Authenticate found for the request.
func CurrentUser(ctx context.Context) (store.User, bool) {
	user, ok := ctx.Value(userKey{}).(store.User)
	return user, ok
}

// Authenticate resolves the User from the session cookie, when there is a
// valid one, and makes it available through CurrentUser. It never rejects a
// request; RequireUser does.
func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		user, renewed, err := s.resolve(r.Context(), cookie.Value)
		if errors.Is(err, errNoSession) {
			// The session expired or was ended: stop the browser sending it.
			http.SetCookie(w, ClearSessionCookie())
			next.ServeHTTP(w, r)
			return
		}
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if renewed {
			http.SetCookie(w, SessionCookie(cookie.Value))
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey{}, user)))
	})
}

var errNoSession = errors.New("auth: no session")

// resolve returns the User whose session has token, or errNoSession. When
// the session was last used more than touchInterval ago, it moves its last
// use to now, which pushes its expiry 90 days ahead, and reports renewed so
// the cookie gets a new Max-Age too.
func (s *Service) resolve(ctx context.Context, token string) (user store.User, renewed bool, err error) {
	q := store.New(s.db)
	row, err := q.GetSessionByTokenHash(ctx, hashToken(token))
	if errors.Is(err, sql.ErrNoRows) {
		return store.User{}, false, errNoSession
	}
	if err != nil {
		return store.User{}, false, fmt.Errorf("auth: find session: %w", err)
	}

	now := s.now()
	idle := now.Sub(row.Session.LastUsedAt)
	if idle >= sessionLifetime {
		if _, err := q.DeleteSession(ctx, store.DeleteSessionParams{ID: row.Session.ID, UserID: row.User.ID}); err != nil {
			return store.User{}, false, fmt.Errorf("auth: delete expired session: %w", err)
		}
		return store.User{}, false, errNoSession
	}
	if idle >= touchInterval {
		if err := q.TouchSession(ctx, store.TouchSessionParams{LastUsedAt: now, ID: row.Session.ID}); err != nil {
			return store.User{}, false, fmt.Errorf("auth: touch session: %w", err)
		}
		renewed = true
	}
	return row.User, renewed, nil
}

// RequireUser answers 401 Unauthorized when Authenticate found no User.
func RequireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := CurrentUser(r.Context()); !ok {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
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
