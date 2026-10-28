package sync

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/ledgers"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// Batch is what a device sends when it empties its queue of UnsyncedRecords.
// Each record carries its id from the client and the contents the User
// entered; Push fills in the author, version and creation time.
type Batch struct {
	Expenses    []store.ExpenseWithShares
	Settlements []store.Settlement
}

// Kind tells an Expense's Result from a Settlement's.
type Kind string

const (
	KindExpense    Kind = "expense"
	KindSettlement Kind = "settlement"
)

// Result says what became of one record of a Batch.
type Result struct {
	ID   string
	Kind Kind
	// Version is the version the record is stored with, when accepted.
	Version int64
	// Rejection says why the record was rejected; nil when accepted.
	Rejection *Rejection
}

// Push stores a Batch of userID's UnsyncedRecords in one write and returns
// one Result per record, Expenses first.
func Push(ctx context.Context, db *sql.DB, userID string, batch Batch) ([]Result, error) {
	now := time.Now().UTC()
	var results []Result
	_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
		results = results[:0]
		p := pusher{q: q, userID: userID, version: version, now: now, ledgers: make(map[string]*ledgers.Snapshot)}
		for _, e := range batch.Expenses {
			r, err := p.expense(ctx, e)
			if err != nil {
				return err
			}
			results = append(results, r)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// pusher stores the records of one Batch, all in the same write.
type pusher struct {
	q       *store.Queries
	userID  string
	version int64
	now     time.Time
	// ledgers caches each ledger of the Batch the User can use; nil for one
	// they cannot.
	ledgers map[string]*ledgers.Snapshot
}

// ledger loads the ledger id, or returns nil if it does not exist, was
// deleted, or the User has no active Member in it.
func (p pusher) ledger(ctx context.Context, id string) (*ledgers.Snapshot, error) {
	if l, ok := p.ledgers[id]; ok {
		return l, nil
	}
	l, err := usableLedger(ctx, p.q, p.userID, id)
	if err != nil {
		return nil, err
	}
	p.ledgers[id] = l
	return l, nil
}

func (p pusher) expense(ctx context.Context, e store.ExpenseWithShares) (Result, error) {
	r := Result{ID: e.ID, Kind: KindExpense}
	stored, err := p.q.GetExpense(ctx, e.ID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return r, fmt.Errorf("get expense %s: %w", e.ID, err)
	case stored.LedgerID == e.LedgerID && stored.CreatedBy == p.userID:
		// The device sent it again, maybe because it never got the answer.
		r.Version = stored.Version
		return r, nil
	}
	ledger, err := p.ledger(ctx, e.LedgerID)
	if err != nil {
		return r, err
	}
	if ledger == nil {
		r.Rejection = &Rejection{ReasonLedgerUnavailable, fmt.Errorf("ledger %s", e.LedgerID)}
		return r, nil
	}
	if stored.ID != "" {
		r.Rejection = &Rejection{ReasonIDConflict, errors.New("the id belongs to another expense")}
		return r, nil
	}
	if err := checkExpense(e); err != nil {
		r.Rejection = &Rejection{ReasonInvalidRecord, err}
		return r, nil
	}
	if err := domain.ValidateExpense(ledger.Domain(), ledgers.Expense(e)); err != nil {
		r.Rejection = rejection(err)
		return r, nil
	}

	e.CreatedBy, e.Version, e.CreatedAt = p.userID, p.version, p.now
	if _, _, err := p.q.CreateExpense(ctx, e); err != nil {
		return r, err
	}
	r.Version = p.version
	return r, nil
}
