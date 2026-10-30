package invites_test

import (
	"bytes"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/invites"
)

var (
	secret = bytes.Repeat([]byte("k"), 32)
	ledger = "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5b"
)

func token(t *testing.T, secret []byte, ledgerID string, generation int64) string {
	t.Helper()
	tok, err := invites.Token(secret, ledgerID, generation)
	if err != nil {
		t.Fatal(err)
	}
	return tok
}

func TestATokenCanBeShownAgainAndNamesItsLedger(t *testing.T) {
	tok := token(t, secret, ledger, 3)

	if again := token(t, secret, ledger, 3); again != tok {
		t.Errorf("the same link gave %q and %q", tok, again)
	}
	if id, ok := invites.LedgerID(tok); !ok || id != ledger {
		t.Errorf("LedgerID = %q, %v; want %s", id, ok, ledger)
	}
	if !invites.Valid(secret, tok, 3) {
		t.Error("the current link is not valid")
	}
	if len(tok) < 16 || len(tok) > 128 {
		t.Errorf("len(token) = %d, want between 16 and 128 as the contract says", len(tok))
	}
}

func TestRegeneratingRevokesThePreviousToken(t *testing.T) {
	old := token(t, secret, ledger, 3)

	if invites.Valid(secret, old, 4) {
		t.Error("the link of generation 3 still works at generation 4")
	}
	if token(t, secret, ledger, 4) == old {
		t.Error("generation 4 gave the same token as generation 3")
	}
}

func TestATokenOnlyWorksWithTheServerSecret(t *testing.T) {
	forged := token(t, bytes.Repeat([]byte("x"), 32), ledger, 3)

	if invites.Valid(secret, forged, 3) {
		t.Error("a token made with another secret is valid")
	}
}

func TestTamperedOrMalformedTokensAreNotValid(t *testing.T) {
	tok := token(t, secret, ledger, 0)
	flipped := []byte(tok)
	flipped[len(flipped)-1] ^= 1
	otherLedger := token(t, secret, "019a2b3c-4d5e-7f60-8a9b-0c1d2e3f4a5c", 0)

	for name, bad := range map[string]string{
		"last character changed":              string(flipped),
		"another ledger's mac on this ledger": tok[:22] + otherLedger[22:],
		"not base64":                          "k3J9xQ2mV7pL8wR4tY6u!!",
		"too short":                           tok[:20],
		"empty":                               "",
	} {
		t.Run(name, func(t *testing.T) {
			if invites.Valid(secret, bad, 0) {
				t.Errorf("Valid(%q) = true", bad)
			}
		})
	}
}

func TestAShortSecretIsRefused(t *testing.T) {
	if _, err := invites.Token([]byte("short"), ledger, 0); err == nil {
		t.Error("Token accepted a 5-byte secret")
	}
	if _, err := invites.Token(secret, "not-a-uuid", 0); err == nil {
		t.Error("Token accepted a ledger id that is not a UUID")
	}
}
