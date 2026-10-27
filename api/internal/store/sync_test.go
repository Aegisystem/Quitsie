package store_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

// seedBusyGroup fills ledger-1 with an Expense, a Settlement and an Activity,
// and adds ledger-2, where only Carla is, and returns the last version.
func seedBusyGroup(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	ctx := context.Background()
	seedGroup(t, db)
	insertUser(t, db, "carla")
	createDinner(t, db, "expense-1")
	return write(t, db, func(q *store.Queries, version int64) error {
		s := cashBack("settlement-1")
		s.Version = version
		if _, _, err := q.CreateSettlement(ctx, s); err != nil {
			return err
		}
		err := q.CreateActivity(ctx, store.CreateActivityParams{
			ID: "activity-1", LedgerID: "ledger-1", UserID: "ana", Kind: "expense_created", Details: "{}", Version: version, CreatedAt: now,
		})
		if err != nil {
			return err
		}
		if err := q.CreateLedger(ctx, newGroup("ledger-2", version)); err != nil {
			return err
		}
		return q.CreateMember(ctx, newMember("member-carla", "ledger-2", ptr("carla"), version))
	})
}

func changesSince(t *testing.T, db *sql.DB, userID string, since int64) store.Changes {
	t.Helper()
	changes, err := store.ChangesSince(context.Background(), db, userID, since)
	if err != nil {
		t.Fatal(err)
	}
	return changes
}

func ids[T any](rows []T, id func(T) string) []string {
	out := []string{}
	for _, r := range rows {
		out = append(out, id(r))
	}
	return out
}

func ledgerIDs(c store.Changes) []string {
	return ids(c.Ledgers, func(l store.Ledger) string { return l.ID })
}

func memberIDs(c store.Changes) []string {
	return ids(c.Members, func(m store.Member) string { return m.ID })
}

func expenseIDs(c store.Changes) []string {
	return ids(c.Expenses, func(e store.ExpenseWithShares) string { return e.ID })
}

func settlementIDs(c store.Changes) []string {
	return ids(c.Settlements, func(s store.Settlement) string { return s.ID })
}

func activityIDs(c store.Changes) []string {
	return ids(c.Activities, func(a store.Activity) string { return a.ID })
}

func TestChangesSinceZeroBringEverythingTheUserCanSee(t *testing.T) {
	db := openMigrated(t)
	last := seedBusyGroup(t, db)

	c := changesSince(t, db, "ana", 0)

	if c.Cursor != last {
		t.Errorf("Cursor = %d, want the last version %d", c.Cursor, last)
	}
	assertIDs(t, "Ledgers", ledgerIDs(c), "ledger-1")
	assertIDs(t, "Members", memberIDs(c), "member-ana", "member-beto")
	assertIDs(t, "Expenses", expenseIDs(c), "expense-1")
	assertIDs(t, "Settlements", settlementIDs(c), "settlement-1")
	assertIDs(t, "Activities", activityIDs(c), "activity-1")
	if len(c.Expenses) == 1 && (len(c.Expenses[0].PaidShares) != 1 || len(c.Expenses[0].OwedShares) != 2) {
		t.Errorf("the Expense came without its shares: %+v", c.Expenses[0])
	}
}
