package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	netmail "net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/mail"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

const (
	codeTTL         = 10 * time.Minute
	maxCodeAttempts = 5
	// maxFailures is how many wrong guesses an email takes in failureWindow,
	// across codes: asking for a new code buys no more. That keeps an account
	// to 3,650 guesses out of a million in a year. Anyone can spend them, so a
	// known device does not count against them and has as many of its own.
	maxFailures   = 10
	failureWindow = 24 * time.Hour
	// maxLiveCodesPerEmail bounds the codes of one email that browsers it does
	// not know can hold at once. One every DefaultSendCooldown never reaches it; it
	// counts in the database, where a restart, which forgets the cooldowns,
	// does not forget it.
	maxLiveCodesPerEmail = int64(codeTTL / DefaultSendCooldown)
)

// CodeFlowCookieName is the cookie that ties a login code to the browser
// that asked for it, so that only that browser can check it.
const CodeFlowCookieName = "__Host-quitsie_code_flow"

var (
	// ErrInvalidEmail means the text is not a bare email address.
	ErrInvalidEmail = errors.New("auth: invalid email")
	// ErrWrongCode means the code is not the one sent to that email.
	ErrWrongCode = errors.New("auth: wrong login code")
	// ErrCodeExpired means this browser has no live code for that email: it
	// expired, was already used, or was asked for elsewhere or never. The User
	// must ask for another.
	ErrCodeExpired = errors.New("auth: login code expired")
	// ErrTooManyAttempts means the code was guessed wrong too many times and
	// no longer works. The User must ask for another.
	ErrTooManyAttempts = errors.New("auth: too many attempts with this login code")
	// ErrRateLimited means too many codes were requested for that email or
	// from that IP in the last hour. RequestCode returns it as a
	// *RateLimitError, which says when to try again.
	ErrRateLimited = errors.New("auth: too many login codes requested")
)

// RateLimitError is ErrRateLimited with how long until another code can be
// requested.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string        { return ErrRateLimited.Error() }
func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimited }

// Browser is who asks for a login code or checks one.
type Browser struct {
	// IP is the address the limits per IP count the browser under.
	IP string
	// Flow and KnownDevices are the values of the cookies named
	// CodeFlowCookieName and KnownDeviceCookieName, empty without them.
	Flow         string
	KnownDevices string
}

// BrowserOf reads the cookies of sign-in that r carries. ip is the address
// the limits count it under.
func BrowserOf(r *http.Request, ip string) Browser {
	return Browser{
		IP:           ip,
		Flow:         cookieValue(r, CodeFlowCookieName),
		KnownDevices: cookieValue(r, KnownDeviceCookieName),
	}
}

// codeFlowCookie returns the cookie that carries flow. It lasts as long as
// the code, and it must outlive the page: an installed PWA can be closed
// while its User reads the email.
func codeFlowCookie(flow string) *http.Cookie {
	return hostCookie(CodeFlowCookieName, flow, codeTTL)
}

// ClearCodeFlowCookie returns a cookie that makes the browser delete the flow
// cookie, once its code signed it in.
func ClearCodeFlowCookie() *http.Cookie {
	return clearHostCookie(CodeFlowCookieName)
}

// RequestCode emails a new 6-digit login code to email and returns the flow
// cookie to set: only this browser can check the code (ADR 0011). The code
// replaces any earlier one of the same browser, and leaves alone the codes
// other browsers asked for.
//
// After a code goes out, the email waits Config.SendCooldown for the next one. A
// browser that email's User signed in from before waits on its own; any
// other one shares the wait of the email, which anybody can spend. A code the
// email service refused costs no wait. Every email also counts against the
// server's budget (see DefaultMailPerHour); when it is spent, RequestCode
// returns a *MailBudgetError.
func (s *Service) RequestCode(ctx context.Context, email string, from Browser) (*http.Cookie, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return nil, err
	}
	known, err := s.knownDevice(ctx, store.New(s.db), email, from.KnownDevices)
	if err != nil {
		return nil, err
	}
	// No point in sending a code that could not be used.
	failures, err := s.failures(ctx, store.New(s.db), known.failuresKey(email))
	if err != nil {
		return nil, err
	}
	if failures.spent() {
		return nil, &RateLimitError{RetryAfter: failures.ResetsAt.Sub(s.now())}
	}
	if err := s.checkMailBudget(ctx); err != nil {
		return nil, err
	}
	quotas := []quota{{"email:" + email, 1}}
	if known.ok() {
		quotas = []quota{{"known-device:" + known.tokenHash, 1}}
	}
	// Only a look: the quotas count the request once nothing else refuses it.
	if wait := s.sends.wait(s.now(), quotas...); wait > 0 {
		return nil, &RateLimitError{RetryAfter: wait}
	}
	code, err := newCode()
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	// A new flow each time: a browser never gets to choose its own.
	flow := rand.Text()

	now := s.now()
	var budget mailBudget
	err = s.inTx(ctx, func(q *store.Queries) error {
		if from.Flow != "" {
			if err := q.DeleteLoginCodeOfFlow(ctx, ptr(hashToken(from.Flow))); err != nil {
				return fmt.Errorf("auth: delete the earlier login code: %w", err)
			}
		}
		// Also forget other people's expired codes, and with them their emails.
		if err := q.DeleteExpiredLoginCodes(ctx, now); err != nil {
			return fmt.Errorf("auth: delete expired login codes: %w", err)
		}
		if err := q.DeleteEndedLoginFailures(ctx, now); err != nil {
			return fmt.Errorf("auth: delete ended login failures: %w", err)
		}
		if !known.ok() {
			live, err := q.CountLiveLoginCodes(ctx, store.CountLiveLoginCodesParams{Email: email, ExpiresAt: now})
			if err != nil {
				return fmt.Errorf("auth: count live login codes: %w", err)
			}
			if live >= maxLiveCodesPerEmail {
				return &RateLimitError{RetryAfter: codeTTL}
			}
		}
		var err error
		if budget, err = s.spendMail(ctx, q, now); err != nil {
			return err
		}
		// Last of the checks, so that a request the others refuse spends none
		// of the sender's quotas. It looks again: another request of the same
		// sender may have got in since the look above.
		if ok, retryAfter := s.sends.allow(now, quotas...); !ok {
			return &RateLimitError{RetryAfter: retryAfter}
		}
		err = q.CreateLoginCode(ctx, store.CreateLoginCodeParams{
			ID:        id.String(),
			Email:     email,
			CodeHash:  hashCode(id.String(), code),
			FlowHash:  ptr(hashToken(flow)),
			ExpiresAt: now.Add(codeTTL),
			CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("auth: create login code: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := s.mail.Send(ctx, codeMessage(email, code)); err != nil {
		// Nobody got this code, so it could only ever be guessed, and its
		// owner would never hear of it. Nor did it cost an email, nor does it
		// make the sender wait. Requests for an address the email service
		// keeps refusing are bounded only by the limit of requests per IP
		// (httpapi/limits.go).
		s.sends.forget(now, quotas...)
		q, ctx := store.New(s.db), context.WithoutCancel(ctx)
		cleanup := errors.Join(q.DeleteLoginCodeOfFlow(ctx, ptr(hashToken(flow))), q.UncountMailSend(ctx, budget.hour))
		return nil, fmt.Errorf("auth: send login code: %w", errors.Join(err, cleanup))
	}
	if budget.wait(now) > 0 { // this email spent the last of the hour or the day
		s.reportMailBudgetSpent(ctx, budget, now)
	}
	return codeFlowCookie(flow), nil
}

// VerifyCode signs in the User with this email, creating it if needed, when
// code is the last one sent to it at the request of this same browser. The
// limit per IP stops one sender from spreading guesses over many emails. A
// wrong guess counts against the code, and against the browser's known device
// or else against the email.
func (s *Service) VerifyCode(ctx context.Context, email, code, device string, from Browser) (SignIn, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return SignIn{}, err
	}
	if ok, retryAfter := s.verifications.allow(s.now(), quota{"ip:" + from.IP, maxVerificationsPerIP}); !ok {
		return SignIn{}, &RateLimitError{RetryAfter: retryAfter}
	}
	if from.Flow == "" {
		return SignIn{}, ErrCodeExpired
	}
	var signIn SignIn
	// A wrong code must still count the attempt, so it commits and reports
	// the failure in wrong instead of rolling back.
	var wrong error
	err = s.inTx(ctx, func(q *store.Queries) error {
		lc, err := q.GetLoginCodeOfFlow(ctx, ptr(hashToken(from.Flow)))
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCodeExpired
		}
		if err != nil {
			return fmt.Errorf("auth: find login code: %w", err)
		}
		if lc.Email != email || !s.now().Before(lc.ExpiresAt) {
			return ErrCodeExpired
		}
		known, err := s.knownDevice(ctx, q, email, from.KnownDevices)
		if err != nil {
			return err
		}
		failures, err := s.failures(ctx, q, known.failuresKey(email))
		if err != nil {
			return err
		}
		if lc.Attempts >= maxCodeAttempts || failures.spent() {
			return ErrTooManyAttempts
		}
		if subtle.ConstantTimeCompare([]byte(hashCode(lc.ID, code)), []byte(lc.CodeHash)) != 1 {
			wrong = ErrWrongCode
			if err := q.CountLoginCodeAttempt(ctx, lc.ID); err != nil {
				return fmt.Errorf("auth: count attempt: %w", err)
			}
			return q.SetLoginFailures(ctx, failures.plusOne(s.now()))
		}
		// Signed in: the codes other browsers asked for have no use left.
		if err := q.DeleteLoginCodes(ctx, email); err != nil {
			return fmt.Errorf("auth: delete login codes: %w", err)
		}

		user, created, err := s.userByEmail(ctx, q, email)
		if err != nil {
			return err
		}
		token, err := s.createSession(ctx, q, user.ID, device)
		if err != nil {
			return err
		}
		knownDevices, err := s.rememberDevice(ctx, q, user.ID, from.KnownDevices)
		if err != nil {
			return err
		}
		signIn = SignIn{User: user, Token: token, KnownDevices: knownDevices, NewUser: created}
		return nil
	})
	if err != nil {
		return SignIn{}, err
	}
	if wrong != nil {
		return SignIn{}, wrong
	}
	return signIn, nil
}

// failures are the wrong guesses at the codes of one email, or of one known
// device, in the current window.
type failures store.SetLoginFailuresParams

func (f failures) spent() bool { return f.Failures >= maxFailures }

func (f failures) plusOne(now time.Time) store.SetLoginFailuresParams {
	if f.Failures == 0 {
		f.ResetsAt = now.Add(failureWindow)
	}
	f.Failures++
	return store.SetLoginFailuresParams(f)
}

// failures reads the failures counted under key, a hash from failuresKey,
// which are none once its window ended.
func (s *Service) failures(ctx context.Context, q *store.Queries, key string) (failures, error) {
	f := failures{EmailHash: key}
	row, err := q.GetLoginFailures(ctx, f.EmailHash)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return f, nil
	case err != nil:
		return failures{}, fmt.Errorf("auth: find login failures: %w", err)
	case !row.ResetsAt.After(s.now()):
		return f, nil
	}
	f.Failures, f.ResetsAt = row.Failures, row.ResetsAt
	return f, nil
}

func hashEmail(email string) string {
	sum := sha256.Sum256([]byte(email))
	return hex.EncodeToString(sum[:])
}

// NormalizeEmail lower-cases a bare address such as "Ana@Example.com" and
// rejects anything else, including display names and lists.
func NormalizeEmail(email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	addr, err := netmail.ParseAddress(email)
	if err != nil || addr.Name != "" || addr.Address != email || len(email) > 254 {
		return "", ErrInvalidEmail
	}
	return email, nil
}

// newCode returns 6 uniformly random digits.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("auth: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hashCode salts the code with its row id. A stolen hash can still be brute
// forced (there are only a million codes), but only while the code lives.
func hashCode(id, code string) string {
	sum := sha256.Sum256([]byte(id + ":" + code))
	return hex.EncodeToString(sum[:])
}

func ptr[T any](v T) *T { return &v }

func codeMessage(to, code string) mail.Message {
	return mail.Message{
		To:      to,
		Subject: "Tu código de Quitsie: " + code,
		Text: "Tu código para entrar en Quitsie es " + code + ".\n\n" +
			"Caduca en 10 minutos. Si no lo pediste, puedes ignorar este correo.",
	}
}
