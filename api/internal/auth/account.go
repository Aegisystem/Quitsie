package auth

import (
	"context"
	"fmt"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// DeleteUser deletes the User's account. Their Members and Friends become
// pending again with their names, so nobody else's Balances change, and
// every session they had ends. Their email and Google account are freed:
// signing in with either again creates a new User.
func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	// Unlinking Members changes ledgers, so it takes a version for sync.
	_, err := store.Write(ctx, s.db, func(q *store.Queries, version int64) error {
		if err := q.UnlinkMembersOfUser(ctx, store.UnlinkMembersOfUserParams{UserID: userID, Version: version}); err != nil {
			return fmt.Errorf("auth: unlink members: %w", err)
		}
		if err := q.DeleteSessionsOfUser(ctx, userID); err != nil {
			return fmt.Errorf("auth: delete sessions: %w", err)
		}
		now := s.now()
		if err := q.DeleteUser(ctx, store.DeleteUserParams{DeletedAt: &now, ID: userID}); err != nil {
			return fmt.Errorf("auth: delete user: %w", err)
		}
		return nil
	})
	return err
}
