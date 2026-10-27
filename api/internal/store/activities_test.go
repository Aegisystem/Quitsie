package store_test

import (
	"context"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

func TestActivitiesOfALedgerAreListedNewestFirst(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	for _, kind := range []string{"expense_created", "expense_deleted"} {
		write(t, db, func(q *store.Queries, version int64) error {
			return q.CreateActivity(ctx, store.CreateActivityParams{
				ID:        kind,
				LedgerID:  "ledger-1",
				UserID:    "ana",
				Kind:      kind,
				Details:   `{"expense_id":"expense-1"}`,
				Version:   version,
				CreatedAt: now,
			})
		})
	}

	activities, err := store.New(db).ListActivities(ctx, "ledger-1")
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, a := range activities {
		kinds = append(kinds, a.Kind)
	}
	if len(kinds) != 2 || kinds[0] != "expense_deleted" || kinds[1] != "expense_created" {
		t.Errorf("ListActivities kinds = %v, want [expense_deleted expense_created]", kinds)
	}
}
