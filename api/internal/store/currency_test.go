package store_test

import (
	"context"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

func TestRelabelCurrencyKeepsTheNumbersOfEveryAmountInTheLedger(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	q := store.New(db)
	seedGroup(t, db)
	createDinner(t, db, "live")
	createDinner(t, db, "deleted")
	write(t, db, func(q *store.Queries, version int64) error {
		if _, err := q.DeleteExpense(ctx, store.DeleteExpenseParams{ID: "deleted", DeletedAt: &now, Version: version}); err != nil {
			return err
		}
		s := cashBack("settlement-1")
		s.Version = version
		_, _, err := q.CreateSettlement(ctx, s)
		return err
	})

	version := write(t, db, func(q *store.Queries, version int64) error {
		return q.RelabelCurrency(ctx, "ledger-1", "USD", version)
	})

	ledger, err := q.GetLedger(ctx, "ledger-1")
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Currency == nil || *ledger.Currency != "USD" || ledger.Version != version {
		t.Errorf("relabeled ledger = %+v, want USD with version %d", ledger, version)
	}
	for _, id := range []string{"live", "deleted"} {
		e, err := q.GetExpense(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if e.Currency != "USD" || e.Total != 9000 || e.Version != version {
			t.Errorf("relabeled Expense %s = %+v, want 9000 USD with version %d", id, e, version)
		}
	}
	s, err := q.GetSettlement(ctx, "settlement-1")
	if err != nil {
		t.Fatal(err)
	}
	if s.Currency != "USD" || s.Amount != 3000 || s.Version != version {
		t.Errorf("relabeled Settlement = %+v, want 3000 USD with version %d", s, version)
	}
}
