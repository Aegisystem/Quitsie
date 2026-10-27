package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

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
