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
)

// RequestCode emails a new 6-digit login code to email. The code replaces
// any earlier one for the same email.
func (s *Service) RequestCode(ctx context.Context, email, ip string) error {
	email, err := normalizeEmail(email)
	if err != nil {
		return err
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
	err = s.inTx(ctx, func(q *store.Queries) error {
		if err := q.DeleteLoginCodes(ctx, email); err != nil {
			return fmt.Errorf("auth: delete login codes: %w", err)
		}
		err := q.CreateLoginCode(ctx, store.CreateLoginCodeParams{
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
		return fmt.Errorf("auth: send login code: %w", err)
	}
	return nil
}

// VerifyCode signs in the User with this email, creating it if needed, when
// code is the last one sent to it.
func (s *Service) VerifyCode(ctx context.Context, email, code, device string) (SignIn, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return SignIn{}, err
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
		if lc.Attempts >= maxCodeAttempts {
			return ErrTooManyAttempts
		}
		if subtle.ConstantTimeCompare([]byte(hashCode(lc.ID, code)), []byte(lc.CodeHash)) != 1 {
			wrong = ErrWrongCode
			if err := q.CountLoginCodeAttempt(ctx, lc.ID); err != nil {
				return fmt.Errorf("auth: count attempt: %w", err)
			}
			return nil
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

// normalizeEmail lower-cases a bare address such as "Ana@Example.com" and
// rejects anything else, including display names and lists.
func normalizeEmail(email string) (string, error) {
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
