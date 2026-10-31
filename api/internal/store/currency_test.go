package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

// 90.00 COP are 9000 minor units and 90 CLP are 90: the number people read
// stays, so the stored one changes with the decimals of the currency.
func TestRelabelCurrencyRewritesTheMinorUnitsOfACurrencyWithFewerDecimals(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	q := store.New(db)
	seedGroup(t, db)
	createDinner(t, db, "weights")
	write(t, db, func(q *store.Queries, version int64) error {
		adjusted := dinner("adjusted")
		adjusted.SplitMethod = "adjustment"
		adjusted.OwedShares[0].SplitValue = ptr[int64](1500)
		adjusted.OwedShares[1].SplitValue = ptr[int64](-1500)
		adjusted.Version = version
		if _, _, err := q.CreateExpense(ctx, adjusted); err != nil {
			return err
		}
		if _, err := q.DeleteExpense(ctx, store.DeleteExpenseParams{ID: "adjusted", DeletedAt: &now, Version: version}); err != nil {
			return err
		}
		s := cashBack("settlement-1")
		s.Version = version
		_, _, err := q.CreateSettlement(ctx, s)
		return err
	})
	// Another ledger with the same amounts must stay as it is.
	write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, newGroup("ledger-2", version)); err != nil {
			return err
		}
		other := dinner("other")
		other.LedgerID = "ledger-2"
		other.Version = version
		_, _, err := q.CreateExpense(ctx, other)
		return err
	})

	write(t, db, func(q *store.Queries, version int64) error {
		return q.RelabelCurrency(ctx, "ledger-1", "CLP", version)
	})

	for id, want := range map[string]string{
		"weights":  "CLP 90 paid[90] owed[60:2 30:1]",
		"adjusted": "CLP 90 paid[90] owed[60:15 30:-15]",
		"other":    "COP 9000 paid[9000] owed[6000:2 3000:1]",
	} {
		e, err := q.GetExpenseWithShares(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprintf("%s %d paid[%d] owed[%d:%d %d:%d]", e.Currency, e.Total, e.PaidShares[0].Amount,
			e.OwedShares[0].Amount, *e.OwedShares[0].SplitValue, e.OwedShares[1].Amount, *e.OwedShares[1].SplitValue)
		if got != want {
			t.Errorf("Expense %s = %s, want %s", id, got, want)
		}
	}
	s, err := q.GetSettlement(ctx, "settlement-1")
	if err != nil {
		t.Fatal(err)
	}
	if s.Currency != "CLP" || s.Amount != 30 {
		t.Errorf("Settlement = %d %s, want 30 CLP", s.Amount, s.Currency)
	}
}

func TestRelabelCurrencyAddsMinorUnitsForACurrencyWithMoreDecimals(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	createDinner(t, db, "dinner")

	write(t, db, func(q *store.Queries, version int64) error {
		return q.RelabelCurrency(ctx, "ledger-1", "KWD", version)
	})

	e, err := store.New(db).GetExpenseWithShares(ctx, "dinner")
	if err != nil {
		t.Fatal(err)
	}
	if e.Total != 90000 || e.PaidShares[0].Amount != 90000 || e.OwedShares[0].Amount != 60000 {
		t.Errorf("Expense = %+v, want every amount times 10", e)
	}
}

func TestOnlyALiveGroupCanBeRelabeled(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, store.CreateLedgerParams{ID: "friendship-1", Kind: "friendship", Version: version, CreatedAt: now}); err != nil {
			return err
		}
		if err := q.CreateLedger(ctx, newGroup("deleted-group", version)); err != nil {
			return err
		}
		_, err := q.DeleteLedger(ctx, store.DeleteLedgerParams{ID: "deleted-group", DeletedAt: &now, Version: version})
		return err
	})

	for _, id := range []string{"friendship-1", "deleted-group", "no-such-ledger"} {
		_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
			return q.RelabelCurrency(ctx, id, "USD", version)
		})

		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("relabeling %s: err = %v, want sql.ErrNoRows", id, err)
		}
	}
}
