package store_test

import (
	"context"
	"database/sql"
	"errors"
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
