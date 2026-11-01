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
	// maxFailuresPerEmail is how many wrong guesses an email takes in
	// failureWindow, across codes: asking for a new code buys no more. That
	// keeps an account to 3,650 guesses out of a million in a year, at the
	// price that anyone can lock an email out of codes for a day.
	maxFailuresPerEmail = 10
	failureWindow       = 24 * time.Hour
)

var (
	// ErrInvalidEmail means the text is not a bare email address.
	ErrInvalidEmail = errors.New("auth: invalid email")
	// ErrWrongCode means the code is not the one sent to that email.
	ErrWrongCode = errors.New("auth: wrong login code")
	// ErrCodeExpired means there is no live code for that email: it expired,
	// was already used, or was never requested. The User must ask for another.
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

// RequestCode emails a new 6-digit login code to email. The code replaces
// any earlier one for the same email. ip is the client's address, used to
// limit how many codes one sender can request. Every email counts against
// the server's budget (see DefaultMailPerHour); when it is spent, RequestCode
// returns a *MailBudgetError.
func (s *Service) RequestCode(ctx context.Context, email, ip string) error {
	email, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	// No point in sending a code that could not be used.
	failures, err := s.failures(ctx, store.New(s.db), email)
	if err != nil {
		return err
	}
	if failures.spent() {
		return &RateLimitError{RetryAfter: failures.ResetsAt.Sub(s.now())}
	}
	if err := s.checkMailBudget(ctx); err != nil {
		return err
	}
	if ok, retryAfter := s.sends.allow(s.now(), quota{"email:" + email, maxSendsPerMail}, quota{"ip:" + ip, maxSendsPerIP}); !ok {
		return &RateLimitError{RetryAfter: retryAfter}
	}
	code, err := newCode()
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}

	now := s.now()
	var budgetHour time.Time
	var lastOfBudget bool
	err = s.inTx(ctx, func(q *store.Queries) error {
		if err := q.DeleteLoginCodes(ctx, email); err != nil {
			return fmt.Errorf("auth: delete login codes: %w", err)
		}
		// Also forget other people's expired codes, and with them their emails.
		if err := q.DeleteExpiredLoginCodes(ctx, now); err != nil {
			return fmt.Errorf("auth: delete expired login codes: %w", err)
		}
		if err := q.DeleteEndedLoginFailures(ctx, now); err != nil {
			return fmt.Errorf("auth: delete ended login failures: %w", err)
		}
		var err error
		if budgetHour, lastOfBudget, err = s.spendMail(ctx, q); err != nil {
			return err
		}
		err = q.CreateLoginCode(ctx, store.CreateLoginCodeParams{
			ID:        id.String(),
			Email:     email,
			CodeHash:  hashCode(id.String(), code),
			ExpiresAt: now.Add(codeTTL),
			CreatedAt: now,
		})
		if err != nil {
			return fmt.Errorf("auth: create login code: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err := s.mail.Send(ctx, codeMessage(email, code)); err != nil {
		// Nobody got this code, so it could only ever be guessed, and its
		// owner would never hear of it. Nor did it cost an email.
		q, ctx := store.New(s.db), context.WithoutCancel(ctx)
		cleanup := errors.Join(q.DeleteLoginCodes(ctx, email), q.UncountMailSend(ctx, budgetHour))
		return fmt.Errorf("auth: send login code: %w", errors.Join(err, cleanup))
	}
	if lastOfBudget {
		s.reportMailBudgetSpent(ctx)
	}
	return nil
}

// VerifyCode signs in the User with this email, creating it if needed, when
// code is the last one sent to it. ip is the client's address, used to limit
// how many guesses one sender can spread over many emails.
func (s *Service) VerifyCode(ctx context.Context, email, code, device, ip string) (SignIn, error) {
	email, err := NormalizeEmail(email)
	if err != nil {
		return SignIn{}, err
	}
	if ok, retryAfter := s.verifications.allow(s.now(), quota{"ip:" + ip, maxVerificationsPerIP}); !ok {
		return SignIn{}, &RateLimitError{RetryAfter: retryAfter}
	}
	var signIn SignIn
	// A wrong code must still count the attempt, so it commits and reports
	// the failure in wrong instead of rolling back.
	var wrong error
	err = s.inTx(ctx, func(q *store.Queries) error {
		lc, err := q.GetLatestLoginCode(ctx, email)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrCodeExpired
		}
		if err != nil {
			return fmt.Errorf("auth: find login code: %w", err)
		}
		if !s.now().Before(lc.ExpiresAt) {
			return ErrCodeExpired
		}
		failures, err := s.failures(ctx, q, email)
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
		signIn = SignIn{User: user, Token: token, NewUser: created}
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

// failures are the wrong guesses at the codes of one email in the current
// window.
type failures store.SetLoginFailuresParams

func (f failures) spent() bool { return f.Failures >= maxFailuresPerEmail }

func (f failures) plusOne(now time.Time) store.SetLoginFailuresParams {
	if f.Failures == 0 {
		f.ResetsAt = now.Add(failureWindow)
	}
	f.Failures++
	return store.SetLoginFailuresParams(f)
}

// failures reads the failures of email, which are none once its window ended.
func (s *Service) failures(ctx context.Context, q *store.Queries, email string) (failures, error) {
	f := failures{EmailHash: hashEmail(email)}
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

func codeMessage(to, code string) mail.Message {
	return mail.Message{
		To:      to,
		Subject: "Tu código de Quitsie: " + code,
		Text: "Tu código para entrar en Quitsie es " + code + ".\n\n" +
			"Caduca en 10 minutos. Si no lo pediste, puedes ignorar este correo.",
	}
}
