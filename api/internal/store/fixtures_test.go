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
