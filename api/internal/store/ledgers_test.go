package store_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

func TestUpdateLedgerChangesNameAndDebtSimplification(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	write(t, db, func(q *store.Queries, version int64) error {
		return q.CreateLedger(ctx, newGroup("ledger-1", version))
	})

	var updated store.Ledger
	version := write(t, db, func(q *store.Queries, version int64) (err error) {
		updated, err = q.UpdateLedger(ctx, store.UpdateLedgerParams{
			ID:            "ledger-1",
			Name:          "Cartagena 2026",
			SimplifyDebts: false,
			Version:       version,
		})
		return err
	})

	if updated.Name != "Cartagena 2026" || updated.SimplifyDebts || updated.Version != version {
		t.Errorf("UpdateLedger = %+v, want the new name, no simplification and version %d", updated, version)
	}
}

func TestDeleteLedgerKeepsItAsAVersionedDeletion(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	write(t, db, func(q *store.Queries, version int64) error {
		return q.CreateLedger(ctx, newGroup("ledger-1", version))
	})

	version := write(t, db, func(q *store.Queries, version int64) error {
		_, err := q.DeleteLedger(ctx, store.DeleteLedgerParams{ID: "ledger-1", DeletedAt: &now, Version: version})
		return err
	})

	ledger, err := store.New(db).GetLedger(ctx, "ledger-1")
	if err != nil {
		t.Fatal(err)
	}
	if ledger.DeletedAt == nil || !ledger.DeletedAt.Equal(now) || ledger.Version != version {
		t.Errorf("deleted ledger = %+v, want deleted at %v with version %d", ledger, now, version)
	}
	_, err = store.Write(ctx, db, func(q *store.Queries, version int64) error {
		_, err := q.UpdateLedger(ctx, store.UpdateLedgerParams{ID: "ledger-1", Name: "Otro", Version: version})
		return err
	})
	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdateLedger on a deleted ledger: err = %v, want sql.ErrNoRows", err)
	}
}

func TestRegenerateInviteLinkMovesToTheNextGeneration(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	write(t, db, func(q *store.Queries, version int64) error {
		return q.CreateLedger(ctx, newGroup("ledger-1", version))
	})

	var regenerated store.Ledger
	version := write(t, db, func(q *store.Queries, version int64) (err error) {
		regenerated, err = q.RegenerateInviteLink(ctx, store.RegenerateInviteLinkParams{ID: "ledger-1", Version: version})
		return err
	})

	if regenerated.InviteLinkGeneration != 1 || regenerated.Version != version {
		t.Errorf("regenerated ledger = %+v, want generation 1 with version %d", regenerated, version)
	}
}

func TestRegenerateInviteLinkOfADeletedLedgerChangesNothing(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, newGroup("ledger-1", version)); err != nil {
			return err
		}
		_, err := q.DeleteLedger(ctx, store.DeleteLedgerParams{ID: "ledger-1", DeletedAt: &now, Version: version})
		return err
	})

	_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
		_, err := q.RegenerateInviteLink(ctx, store.RegenerateInviteLinkParams{ID: "ledger-1", Version: version})
		return err
	})

	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("RegenerateInviteLink on a deleted ledger: err = %v, want sql.ErrNoRows", err)
	}
}

func TestScaleLedgerAmountsRewritesEveryAmountOfTheLedgerInOtherMinorUnits(t *testing.T) {
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
		return q.ScaleLedgerAmounts(ctx, "ledger-1", 2, 0)
	})

	for id, want := range map[string]string{
		"weights":  "90 paid[90] owed[60:2 30:1]",
		"adjusted": "90 paid[90] owed[60:15 30:-15]",
		"other":    "9000 paid[9000] owed[6000:2 3000:1]",
	} {
		e, err := q.GetExpenseWithShares(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		got := fmt.Sprintf("%d paid[%d] owed[%d:%d %d:%d]", e.Total, e.PaidShares[0].Amount,
			e.OwedShares[0].Amount, *e.OwedShares[0].SplitValue, e.OwedShares[1].Amount, *e.OwedShares[1].SplitValue)
		if got != want {
			t.Errorf("Expense %s = %s, want %s", id, got, want)
		}
	}
	s, err := q.GetSettlement(ctx, "settlement-1")
	if err != nil {
		t.Fatal(err)
	}
	if s.Amount != 30 {
		t.Errorf("Settlement amount = %d, want 30", s.Amount)
	}
}

func TestScaleLedgerAmountsAddsMinorUnitsForACurrencyWithMoreDecimals(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	createDinner(t, db, "dinner")

	write(t, db, func(q *store.Queries, version int64) error {
		return q.ScaleLedgerAmounts(ctx, "ledger-1", 2, 3)
	})

	e, err := store.New(db).GetExpenseWithShares(ctx, "dinner")
	if err != nil {
		t.Fatal(err)
	}
	if e.Total != 90000 || e.PaidShares[0].Amount != 90000 || e.OwedShares[0].Amount != 60000 {
		t.Errorf("Expense = %+v, want every amount times 10", e)
	}
}
