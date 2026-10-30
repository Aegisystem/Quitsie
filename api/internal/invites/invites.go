// Package invites derives the tokens of Invite links (ADR 0010). A token is
// the ledger id followed by an HMAC of the server's secret over that id and
// the ledger's link generation, so the database never stores it and the link
// can be shown again at any time. Regenerating the link is incrementing the
// generation, which revokes the previous token.
package invites

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// MinSecretLength is the shortest secret Token accepts, in bytes.
const MinSecretLength = 32

// macLength is how much of the HMAC-SHA256 goes in a token: 128 bits.
const macLength = 16

var encoding = base64.RawURLEncoding

// ErrShortSecret means the server's secret is missing or too short to sign
// links that nobody can forge.
var ErrShortSecret = fmt.Errorf("invites: the secret must have at least %d bytes", MinSecretLength)

// Token returns the token of the Invite link of ledgerID at generation.
func Token(secret []byte, ledgerID string, generation int64) (string, error) {
	if len(secret) < MinSecretLength {
		return "", ErrShortSecret
	}
	id, err := uuid.Parse(ledgerID)
	if err != nil {
		return "", fmt.Errorf("invites: ledger id: %w", err)
	}
	return encoding.EncodeToString(append(id[:], mac(secret, id, generation)...)), nil
}

// LedgerID reads which ledger a token is for, without checking it: load
// that ledger's generation and pass it to Valid.
func LedgerID(token string) (string, bool) {
	id, _, err := split(token)
	if err != nil {
		return "", false
	}
	return id.String(), true
}

// Valid reports whether token is the Invite link of its ledger at
// generation, signed with secret.
func Valid(secret []byte, token string, generation int64) bool {
	if len(secret) < MinSecretLength {
		return false
	}
	id, got, err := split(token)
	if err != nil {
		return false
	}
	return hmac.Equal(got, mac(secret, id, generation))
}

func split(token string) (uuid.UUID, []byte, error) {
	raw, err := encoding.DecodeString(token)
	if err != nil {
		return uuid.UUID{}, nil, err
	}
	if len(raw) != len(uuid.UUID{})+macLength {
		return uuid.UUID{}, nil, errors.New("invites: wrong token length")
	}
	return uuid.UUID(raw[:16]), raw[16:], nil
}

func mac(secret []byte, ledgerID uuid.UUID, generation int64) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte("quitsie invite link\x00"))
	h.Write(ledgerID[:])
	h.Write(binary.BigEndian.AppendUint64(nil, uint64(generation)))
	return h.Sum(nil)[:macLength]
}
