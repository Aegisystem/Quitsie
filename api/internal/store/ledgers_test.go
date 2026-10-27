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

func TestInviteLinkFindsItsLedgerUntilRegeneratedOrRevoked(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	q := store.New(db)
	setInviteLink := func(hash *string) {
		t.Helper()
		write(t, db, func(q *store.Queries, version int64) error {
			_, err := q.SetLedgerInviteLink(ctx, store.SetLedgerInviteLinkParams{ID: "ledger-1", InviteLinkHash: hash, Version: version})
			return err
		})
	}
	write(t, db, func(q *store.Queries, version int64) error {
		return q.CreateLedger(ctx, newGroup("ledger-1", version))
	})

	setInviteLink(ptr("hash-1"))
	if ledger, err := q.GetLedgerByInviteLink(ctx, "hash-1"); err != nil || ledger.ID != "ledger-1" {
		t.Errorf("GetLedgerByInviteLink(hash-1) = %q, %v; want ledger-1", ledger.ID, err)
	}

	setInviteLink(ptr("hash-2"))
	if _, err := q.GetLedgerByInviteLink(ctx, "hash-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("regenerated link still works: err = %v, want sql.ErrNoRows", err)
	}

	setInviteLink(nil)
	if _, err := q.GetLedgerByInviteLink(ctx, "hash-2"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("revoked link still works: err = %v, want sql.ErrNoRows", err)
	}
}

func TestInviteLinkOfADeletedLedgerFindsNothing(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, newGroup("ledger-1", version)); err != nil {
			return err
		}
		if _, err := q.SetLedgerInviteLink(ctx, store.SetLedgerInviteLinkParams{ID: "ledger-1", InviteLinkHash: ptr("hash-1"), Version: version}); err != nil {
			return err
		}
		_, err := q.DeleteLedger(ctx, store.DeleteLedgerParams{ID: "ledger-1", DeletedAt: &now, Version: version})
		return err
	})

	if _, err := store.New(db).GetLedgerByInviteLink(ctx, "hash-1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("GetLedgerByInviteLink on a deleted ledger: err = %v, want sql.ErrNoRows", err)
	}
}
