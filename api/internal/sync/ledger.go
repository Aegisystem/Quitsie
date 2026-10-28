package sync

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/ledgers"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// usableLedger loads the ledger id with its Members and records, or returns
// nil if it does not exist, was deleted, or userID has no active Member in it:
// every read and write is limited to those ledgers (docs/arquitectura.md).
func usableLedger(ctx context.Context, q *store.Queries, userID, id string) (*ledgers.Snapshot, error) {
	l, err := ledgers.Load(ctx, q, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if l.Ledger.DeletedAt != nil {
		return nil, nil
	}
	for _, m := range l.Members {
		if m.UserID != nil && *m.UserID == userID && m.Status == string(domain.MemberActive) {
			return &l, nil
		}
	}
	return nil, nil
}
