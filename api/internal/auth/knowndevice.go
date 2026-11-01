package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// A known device is a browser that signed in to a User before (ADR 0011).
// Anybody can spend the limits per email of a User's codes, so the browsers
// that User already signed in from skip them, and have smaller limits of
// their own that only they can spend. Signing out leaves the browser known.

// KnownDeviceCookieName is the cookie that tells sign-in which Users a
// browser signed in to before. It holds one token per User, and the database
// keeps only their hashes.
const KnownDeviceCookieName = "__Host-quitsie_known_device"

const (
	// knownDeviceLifetime is how long a browser stays known without signing
	// in. Browsers keep a cookie 400 days at most.
	knownDeviceLifetime = 365 * 24 * time.Hour
	// maxKnownDevicesPerUser bounds the rows of one User; the ones used
	// longest ago go first.
	maxKnownDevicesPerUser = 20
	// maxKnownDevicesPerBrowser is how many Users one browser stays known to,
	// such as the people of a household that share a laptop.
	maxKnownDevicesPerBrowser = 5
	// knownDeviceSweepInterval is how often sign-in looks for the known
	// devices of every User that went knownDeviceLifetime unused.
	knownDeviceSweepInterval = 24 * time.Hour
)

// KnownDeviceCookie returns the cookie that carries the value of
// SignIn.KnownDevices.
func KnownDeviceCookie(value string) *http.Cookie {
	return hostCookie(KnownDeviceCookieName, value, knownDeviceLifetime)
}

// knownDeviceTokens splits the value of the known-device cookie into its
// tokens, newest first. It ignores what is past the most it keeps.
func knownDeviceTokens(value string) []string {
	if value == "" {
		return nil
	}
	tokens := strings.SplitN(value, ".", maxKnownDevicesPerBrowser+1)
	return tokens[:min(len(tokens), maxKnownDevicesPerBrowser)]
}

// knownDevice is the browser's known device for the User of an email: the
// hash of its token, or empty when the browser never signed in to that User.
type knownDevice struct {
	tokenHash string
}

func (d knownDevice) ok() bool { return d.tokenHash != "" }

// failuresKey is where the wrong guesses of this browser at email's codes
// count: those of a known device count apart, so nobody else can spend them.
func (d knownDevice) failuresKey(email string) string {
	if !d.ok() {
		return hashEmail(email)
	}
	return hashToken("known-device:" + d.tokenHash)
}

// knownDevice finds which of the tokens in the browser's known-device cookie
// belongs to the User with this email. A token of another User is no use.
func (s *Service) knownDevice(ctx context.Context, q *store.Queries, email, cookie string) (knownDevice, error) {
	tokens := knownDeviceTokens(cookie)
	if len(tokens) == 0 {
		return knownDevice{}, nil
	}
	user, err := q.GetUserByEmail(ctx, email)
	if errors.Is(err, sql.ErrNoRows) {
		return knownDevice{}, nil
	}
	if err != nil {
		return knownDevice{}, fmt.Errorf("auth: find user: %w", err)
	}
	for _, token := range tokens {
		row, err := q.GetKnownDevice(ctx, hashToken(token))
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return knownDevice{}, fmt.Errorf("auth: find known device: %w", err)
		}
		if row.UserID == user.ID && s.now().Sub(row.LastUsedAt) < knownDeviceLifetime {
			return knownDevice{tokenHash: row.TokenHash}, nil
		}
	}
	return knownDevice{}, nil
}

// rememberDevice records that the browser with this known-device cookie
// signed in to userID, and returns the cookie's new value. The browser keeps
// its token for this User or gets a new one; it never chooses its own, so
// nobody can plant one they know. The tokens of other Users that are still
// known follow.
func (s *Service) rememberDevice(ctx context.Context, q *store.Queries, userID, cookie string) (string, error) {
	now := s.now()
	var mine string
	var others []string
	for _, token := range knownDeviceTokens(cookie) {
		row, err := q.GetKnownDevice(ctx, hashToken(token))
		switch {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return "", fmt.Errorf("auth: find known device: %w", err)
		case row.UserID != userID:
			others = append(others, token)
		case mine == "":
			mine = token
		}
	}

	if mine != "" {
		if err := q.TouchKnownDevice(ctx, store.TouchKnownDeviceParams{LastUsedAt: now, TokenHash: hashToken(mine)}); err != nil {
			return "", fmt.Errorf("auth: touch known device: %w", err)
		}
	} else {
		mine = rand.Text()
		err := q.CreateKnownDevice(ctx, store.CreateKnownDeviceParams{TokenHash: hashToken(mine), UserID: userID, CreatedAt: now, LastUsedAt: now})
		if err != nil {
			return "", fmt.Errorf("auth: create known device: %w", err)
		}
	}
	err := q.DeleteSurplusKnownDevices(ctx, store.DeleteSurplusKnownDevicesParams{UserID: userID, Keep: maxKnownDevicesPerUser})
	if err != nil {
		return "", fmt.Errorf("auth: delete surplus known devices: %w", err)
	}
	if err := s.deleteStaleKnownDevices(ctx, q, now); err != nil {
		return "", err
	}

	tokens := append([]string{mine}, others...)
	return strings.Join(tokens[:min(len(tokens), maxKnownDevicesPerBrowser)], "."), nil
}

// deleteStaleKnownDevices deletes the known devices of every User that went
// knownDeviceLifetime unused: those of a User who never signs in again would
// otherwise stay forever. It reads the whole table, up to 20 rows per User,
// with the write lock held, so it runs once every knownDeviceSweepInterval and
// not on every sign-in, which anyone with a Google account can repeat (finding
// 14 of docs/revision-seguridad.md).
func (s *Service) deleteStaleKnownDevices(ctx context.Context, q *store.Queries, now time.Time) error {
	if !s.deviceSweep.due(now) {
		return nil
	}
	if err := q.DeleteStaleKnownDevices(ctx, now.Add(-knownDeviceLifetime)); err != nil {
		return fmt.Errorf("auth: delete stale known devices: %w", err)
	}
	return nil
}

// periodic tells when a job is due that must not run more than once per
// interval.
type periodic struct {
	interval time.Duration
	mu       sync.Mutex
	last     time.Time
}

// due reports whether interval passed since the job was last due, and if so
// counts it as due now.
func (w *periodic) due(now time.Time) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.last.IsZero() && now.Sub(w.last) < w.interval {
		return false
	}
	w.last = now
	return true
}
