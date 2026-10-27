package store_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

var now = time.Date(2026, 10, 26, 12, 30, 0, 0, time.UTC)

func ptr[T any](v T) *T { return &v }

// write runs fn with store.Write and fails the test if it fails.
func write(t *testing.T, db *sql.DB, fn func(q *store.Queries, version int64) error) int64 {
	t.Helper()
	version, err := store.Write(context.Background(), db, fn)
	if err != nil {
		t.Fatal(err)
	}
	return version
}

func newGroup(id string, version int64) store.CreateLedgerParams {
	return store.CreateLedgerParams{
		ID:            id,
		Kind:          "group",
		Name:          "Viaje a Cartagena",
		Currency:      ptr("COP"),
		SimplifyDebts: true,
		Version:       version,
		CreatedAt:     now,
	}
}

// insertUser adds a User with raw SQL, so these tests do not depend on the
// auth queries in users.sql.
func insertUser(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO users (id, email, name, preferred_currency, created_at) VALUES (?, ?, ?, 'COP', ?)`,
		id, id+"@example.com", id, now)
	if err != nil {
		t.Fatal(err)
	}
}

func newMember(id, ledgerID string, userID *string, version int64) store.CreateMemberParams {
	return store.CreateMemberParams{
		ID:        id,
		LedgerID:  ledgerID,
		UserID:    userID,
		Name:      id,
		Version:   version,
		CreatedAt: now,
	}
}

// seedGroup creates the group ledger-1 with two linked Members, member-ana
// and member-beto, and returns the version it was written with.
func seedGroup(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	ctx := context.Background()
	insertUser(t, db, "ana")
	insertUser(t, db, "beto")
	return write(t, db, func(q *store.Queries, version int64) error {
		if err := q.CreateLedger(ctx, newGroup("ledger-1", version)); err != nil {
			return err
		}
		if err := q.CreateMember(ctx, newMember("member-ana", "ledger-1", ptr("ana"), version)); err != nil {
			return err
		}
		return q.CreateMember(ctx, newMember("member-beto", "ledger-1", ptr("beto"), version))
	})
}

// dinner is an Expense of ledger-1 that Ana paid and split 2:1 with Beto.
func dinner(id string) store.ExpenseWithShares {
	return store.ExpenseWithShares{
		Expense: store.Expense{
			ID:          id,
			LedgerID:    "ledger-1",
			Description: "Cena",
			Date:        time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC),
			Currency:    "COP",
			Total:       9000,
			Category:    ptr("food"),
			SplitMethod: "weights",
			CreatedBy:   "ana",
			CreatedAt:   now,
		},
		PaidShares: []store.PaidShare{
			{ExpenseID: id, MemberID: "member-ana", Amount: 9000},
		},
		OwedShares: []store.OwedShare{
			{ExpenseID: id, MemberID: "member-ana", Amount: 6000, SplitValue: ptr[int64](2)},
			{ExpenseID: id, MemberID: "member-beto", Amount: 3000, SplitValue: ptr[int64](1)},
		},
	}
}

// cashBack is a Settlement of ledger-1 in which Beto pays Ana back.
func cashBack(id string) store.Settlement {
	return store.Settlement{
		ID:           id,
		LedgerID:     "ledger-1",
		FromMemberID: "member-beto",
		ToMemberID:   "member-ana",
		Amount:       3000,
		Currency:     "COP",
		Date:         time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC),
		CreatedBy:    "beto",
		CreatedAt:    now,
	}
}
