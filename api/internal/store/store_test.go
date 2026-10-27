package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

func openMigrated(t *testing.T) *sql.DB {
	t.Helper()
	ctx := context.Background()
	db, err := store.Open(ctx, filepath.Join(t.TempDir(), "quitsie.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := store.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestOpenUsesWAL(t *testing.T) {
	db := openMigrated(t)

	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := openMigrated(t)

	if err := store.Migrate(context.Background(), db); err != nil {
		t.Errorf("second Migrate: %v", err)
	}
}

func TestUserRoundTrip(t *testing.T) {
	ctx := context.Background()
	q := store.New(openMigrated(t))
	createdAt := time.Date(2026, 10, 26, 12, 30, 0, 0, time.UTC)

	err := q.CreateUser(ctx, store.CreateUserParams{
		ID:                "019a2b3c-0000-7000-8000-000000000001",
		Email:             "ana@example.com",
		Name:              "Ana",
		PreferredCurrency: "COP",
		CreatedAt:         createdAt,
	})
	if err != nil {
		t.Fatal(err)
	}

	got, err := q.GetUserByEmail(ctx, "ana@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ana" || got.PreferredCurrency != "COP" || !got.CreatedAt.Equal(createdAt) {
		t.Errorf("GetUserByEmail = %+v", got)
	}
	if got.GoogleSub != nil || got.DeletedAt != nil {
		t.Errorf("optional fields should be nil, got %+v", got)
	}
}

func TestNextVersionCountsUp(t *testing.T) {
	ctx := context.Background()
	q := store.New(openMigrated(t))

	for want := int64(1); want <= 3; want++ {
		got, err := q.NextVersion(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("NextVersion = %d, want %d", got, want)
		}
	}
}

func TestSchemaRejectsNonISOCurrency(t *testing.T) {
	q := store.New(openMigrated(t))

	err := q.CreateUser(context.Background(), store.CreateUserParams{
		ID:                "019a2b3c-0000-7000-8000-000000000002",
		Email:             "beto@example.com",
		Name:              "Beto",
		PreferredCurrency: "usd",
		CreatedAt:         time.Now().UTC(),
	})
	if err == nil {
		t.Error("CreateUser accepted a lower-case currency code")
	}
}

func TestSchemaEnforcesForeignKeys(t *testing.T) {
	db := openMigrated(t)

	_, err := db.Exec(`INSERT INTO sessions (id, user_id, token_hash, device, created_at, last_used_at)
		VALUES ('s1', 'no-such-user', 'hash', 'test', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
	if err == nil {
		t.Error("inserted a session for a user that does not exist")
	}
}
