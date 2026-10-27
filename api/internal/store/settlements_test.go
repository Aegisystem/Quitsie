package store_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

func TestCreateSettlementAgainWithTheSameIDKeepsTheFirstOne(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	var first store.Settlement
	firstVersion := write(t, db, func(q *store.Queries, version int64) (err error) {
		s := cashBack("settlement-1")
		s.Version = version
		first, _, err = q.CreateSettlement(ctx, s)
		return err
	})

	var again store.Settlement
	var created bool
	write(t, db, func(q *store.Queries, version int64) (err error) {
		resent := cashBack("settlement-1")
		resent.Amount = 9999
		resent.Version = version
		again, created, err = q.CreateSettlement(ctx, resent)
		return err
	})

	if first.Amount != 3000 || first.Version != firstVersion {
		t.Errorf("created Settlement = %+v, want 3000 with version %d", first, firstVersion)
	}
	if created || again.Amount != 3000 || again.Version != firstVersion {
		t.Errorf("resent Settlement = %+v (created %v), want the first one kept", again, created)
	}
}

func TestDeleteSettlementKeepsItAsAVersionedDeletion(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	write(t, db, func(q *store.Queries, version int64) error {
		s := cashBack("settlement-1")
		s.Version = version
		_, _, err := q.CreateSettlement(ctx, s)
		return err
	})
	deleteIt := func() (version int64, err error) {
		return store.Write(ctx, db, func(q *store.Queries, version int64) error {
			_, err := q.DeleteSettlement(ctx, store.DeleteSettlementParams{ID: "settlement-1", DeletedAt: &now, Version: version})
			return err
		})
	}

	version, err := deleteIt()
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.New(db).GetSettlement(ctx, "settlement-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.DeletedAt == nil || got.Version != version {
		t.Errorf("deleted Settlement = %+v, want a deletion date and version %d", got, version)
	}
	if _, err := deleteIt(); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("deleting it again: err = %v, want sql.ErrNoRows", err)
	}
}
