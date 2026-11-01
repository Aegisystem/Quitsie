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

func TestChangesSinceACursorBringOnlyLaterChangesIncludingDeletions(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cursor := seedBusyGroup(t, db)
	write(t, db, func(q *store.Queries, version int64) error {
		current, err := q.GetExpense(ctx, "expense-1")
		if err != nil {
			return err
		}
		e := equalSplit("expense-1")
		e.Version = version
		if _, err := q.UpdateExpense(ctx, e, current.Version); err != nil {
			return err
		}
		_, err = q.DeleteSettlement(ctx, store.DeleteSettlementParams{ID: "settlement-1", DeletedAt: &now, Version: version})
		return err
	})

	c := changesSince(t, db, "ana", cursor)

	assertIDs(t, "Ledgers", ledgerIDs(c))
	assertIDs(t, "Members", memberIDs(c))
	assertIDs(t, "Expenses", expenseIDs(c), "expense-1")
	assertIDs(t, "Settlements", settlementIDs(c), "settlement-1")
	assertIDs(t, "Activities", activityIDs(c))
	if len(c.Expenses) == 1 && c.Expenses[0].Total != 10000 {
		t.Errorf("pulled Expense = %+v, want the edited one", c.Expenses[0].Expense)
	}
	if len(c.Settlements) == 1 && c.Settlements[0].DeletedAt == nil {
		t.Errorf("pulled Settlement = %+v, want its deletion date", c.Settlements[0])
	}
}

func TestChangesBringTheWholeLedgerTheUserJustJoined(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedBusyGroup(t, db)
	cursor := write(t, db, func(q *store.Queries, version int64) error {
		return q.CreateMember(ctx, newMember("member-pending", "ledger-1", nil, version))
	})
	write(t, db, func(q *store.Queries, version int64) error {
		_, err := q.ClaimMember(ctx, store.ClaimMemberParams{ID: "member-pending", UserID: "carla", Version: version})
		return err
	})

	c := changesSince(t, db, "carla", cursor)

	assertIDs(t, "Ledgers", ledgerIDs(c), "ledger-1")
	assertIDs(t, "Members", memberIDs(c), "member-ana", "member-beto", "member-pending")
	assertIDs(t, "Expenses", expenseIDs(c), "expense-1")
	assertIDs(t, "Settlements", settlementIDs(c), "settlement-1")
	assertIDs(t, "Activities", activityIDs(c), "activity-1")
}

// page reads one pull of at most about maxRows rows.
func page(t *testing.T, db *sql.DB, userID string, since, maxRows int64) store.Changes {
	t.Helper()
	var c store.Changes
	err := store.Read(context.Background(), db, func(q *store.Queries) (err error) {
		c, err = q.ChangesSince(context.Background(), userID, since, maxRows)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestAPullInPagesBringsTheSameAsOneInOneGo(t *testing.T) {
	db := openMigrated(t)
	seedBusyGroup(t, db)
	createDinner(t, db, "expense-2")
	createDinner(t, db, "expense-3")
	whole := changesSince(t, db, "ana", 0)

	var all store.Changes
	since, pages := int64(0), 0
	for {
		c := page(t, db, "ana", since, 1)
		pages++
		if c.Cursor <= since && c.HasMore {
			t.Fatalf("page %d: cursor %d does not move on from %d", pages, c.Cursor, since)
		}
		all.Ledgers = append(all.Ledgers, c.Ledgers...)
		all.Members = append(all.Members, c.Members...)
		all.Expenses = append(all.Expenses, c.Expenses...)
		all.Settlements = append(all.Settlements, c.Settlements...)
		all.Activities = append(all.Activities, c.Activities...)
		since = c.Cursor
		if !c.HasMore {
			break
		}
	}

	if pages < 2 {
		t.Errorf("%d pages, want the pull to be cut", pages)
	}
	if since != whole.Cursor {
		t.Errorf("last cursor = %d, want %d", since, whole.Cursor)
	}
	assertIDs(t, "Ledgers", ledgerIDs(all), ledgerIDs(whole)...)
	assertIDs(t, "Members", memberIDs(all), memberIDs(whole)...)
	assertIDs(t, "Expenses", expenseIDs(all), expenseIDs(whole)...)
	assertIDs(t, "Settlements", settlementIDs(all), settlementIDs(whole)...)
	assertIDs(t, "Activities", activityIDs(all), activityIDs(whole)...)
}

func TestAPullThatFitsDoesNotAskForMore(t *testing.T) {
	db := openMigrated(t)
	seedBusyGroup(t, db)

	c := page(t, db, "ana", 0, 1000)

	if c.HasMore {
		t.Errorf("HasMore = true, want false")
	}
}

// The rows of a ledger the User just joined are older than the cursor, so they
// have to come with the page of the joining, or no later page would bring them.
func TestAJoinedLedgerArrivesWholeWhateverThePageSize(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedBusyGroup(t, db)
	cursor := write(t, db, func(q *store.Queries, version int64) error {
		return q.CreateMember(ctx, newMember("member-pending", "ledger-1", nil, version))
	})
	write(t, db, func(q *store.Queries, version int64) error {
		_, err := q.ClaimMember(ctx, store.ClaimMemberParams{ID: "member-pending", UserID: "carla", Version: version})
		return err
	})

	c := page(t, db, "carla", cursor, 1)

	assertIDs(t, "Expenses", expenseIDs(c), "expense-1")
	assertIDs(t, "Settlements", settlementIDs(c), "settlement-1")
	assertIDs(t, "Activities", activityIDs(c), "activity-1")
	assertIDs(t, "Members", memberIDs(c), "member-ana", "member-beto", "member-pending")
}

func TestChangesTellTheUserToDropALedgerTheyLeft(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	cursor := seedBusyGroup(t, db)
	write(t, db, func(q *store.Queries, version int64) error {
		_, err := q.MarkMemberFormer(ctx, store.MarkMemberFormerParams{ID: "member-beto", Version: version})
		return err
	})
	createDinner(t, db, "after-leaving")

	c := changesSince(t, db, "beto", cursor)

	assertIDs(t, "DroppedLedgerIDs", c.DroppedLedgerIDs, "ledger-1")
	assertIDs(t, "Members", memberIDs(c))
	assertIDs(t, "Expenses", expenseIDs(c))
}
