package store_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/store"
)

func TestCreateExpenseStoresItWithItsShares(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	want := dinner("expense-1")

	var created bool
	version := write(t, db, func(q *store.Queries, version int64) (err error) {
		e := want
		e.Version = version
		_, created, err = q.CreateExpense(ctx, e)
		return err
	})

	got, err := store.New(db).GetExpenseWithShares(ctx, "expense-1")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Error("CreateExpense reported an existing Expense")
	}
	if got.Description != "Cena" || got.Total != 9000 || got.SplitMethod != "weights" || got.Version != version {
		t.Errorf("stored Expense = %+v, want the dinner with version %d", got.Expense, version)
	}
	if !reflect.DeepEqual(got.PaidShares, want.PaidShares) {
		t.Errorf("PaidShares = %+v, want %+v", got.PaidShares, want.PaidShares)
	}
	if !reflect.DeepEqual(got.OwedShares, want.OwedShares) {
		t.Errorf("OwedShares = %+v, want %+v", got.OwedShares, want.OwedShares)
	}
}

func TestCreateExpenseKeepsAnOwedShareOfZero(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	// One peso split equally: Beto's OwedShare rounds down to zero.
	peso := dinner("expense-1")
	peso.Total, peso.SplitMethod = 1, "equal"
	peso.PaidShares = []store.PaidShare{{ExpenseID: "expense-1", MemberID: "member-ana", Amount: 1}}
	peso.OwedShares = []store.OwedShare{
		{ExpenseID: "expense-1", MemberID: "member-ana", Amount: 1},
		{ExpenseID: "expense-1", MemberID: "member-beto", Amount: 0},
	}

	write(t, db, func(q *store.Queries, version int64) error {
		peso.Version = version
		_, _, err := q.CreateExpense(ctx, peso)
		return err
	})

	got, err := store.New(db).GetExpenseWithShares(ctx, "expense-1")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.OwedShares, peso.OwedShares) {
		t.Errorf("OwedShares = %+v, want %+v", got.OwedShares, peso.OwedShares)
	}
}

func TestCreateExpenseAgainWithTheSameIDKeepsTheFirstOne(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	first := write(t, db, func(q *store.Queries, version int64) error {
		e := dinner("expense-1")
		e.Version = version
		_, _, err := q.CreateExpense(ctx, e)
		return err
	})

	var got store.ExpenseWithShares
	var created bool
	write(t, db, func(q *store.Queries, version int64) (err error) {
		resent := dinner("expense-1")
		resent.Description = "Cena (reenviada)"
		resent.Version = version
		got, created, err = q.CreateExpense(ctx, resent)
		return err
	})

	if created {
		t.Error("CreateExpense created a resent Expense again")
	}
	if got.Description != "Cena" || got.Version != first || len(got.PaidShares) != 1 || len(got.OwedShares) != 2 {
		t.Errorf("CreateExpense of a resent Expense = %+v, want the first one, with version %d", got, first)
	}
}

// createDinner stores dinner(id) and returns its version.
func createDinner(t *testing.T, db *sql.DB, id string) int64 {
	t.Helper()
	return write(t, db, func(q *store.Queries, version int64) error {
		e := dinner(id)
		e.Version = version
		_, _, err := q.CreateExpense(context.Background(), e)
		return err
	})
}

// equalSplit is dinner(id) re-edited: Beto paid half, split in equal parts.
func equalSplit(id string) store.ExpenseWithShares {
	e := dinner(id)
	e.Description = "Cena y postre"
	e.Total = 10000
	e.SplitMethod = "equal"
	e.PaidShares = []store.PaidShare{
		{ExpenseID: id, MemberID: "member-ana", Amount: 5000},
		{ExpenseID: id, MemberID: "member-beto", Amount: 5000},
	}
	e.OwedShares = []store.OwedShare{
		{ExpenseID: id, MemberID: "member-ana", Amount: 5000},
		{ExpenseID: id, MemberID: "member-beto", Amount: 5000},
	}
	return e
}

func TestUpdateExpenseAtTheExpectedVersionReplacesItAndItsShares(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	created := createDinner(t, db, "expense-1")
	want := equalSplit("expense-1")

	version := write(t, db, func(q *store.Queries, version int64) error {
		e := want
		e.Version = version
		_, err := q.UpdateExpense(ctx, e, created)
		return err
	})

	got, err := store.New(db).GetExpenseWithShares(ctx, "expense-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "Cena y postre" || got.Total != 10000 || got.SplitMethod != "equal" || got.Version != version {
		t.Errorf("edited Expense = %+v, want the new contents with version %d", got.Expense, version)
	}
	if !reflect.DeepEqual(got.PaidShares, want.PaidShares) {
		t.Errorf("PaidShares = %+v, want %+v", got.PaidShares, want.PaidShares)
	}
	if !reflect.DeepEqual(got.OwedShares, want.OwedShares) {
		t.Errorf("OwedShares = %+v, want %+v", got.OwedShares, want.OwedShares)
	}
}

func TestUpdateExpenseAtAStaleVersionIsAConflict(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	stale := createDinner(t, db, "expense-1")
	edit := func(description string, expectedVersion int64) error {
		_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
			e := equalSplit("expense-1")
			e.Description = description
			e.Version = version
			_, err := q.UpdateExpense(ctx, e, expectedVersion)
			return err
		})
		return err
	}
	if err := edit("Primera edición", stale); err != nil {
		t.Fatal(err)
	}

	err := edit("Segunda edición", stale)

	if !errors.Is(err, store.ErrVersionConflict) {
		t.Errorf("UpdateExpense at a stale version: err = %v, want store.ErrVersionConflict", err)
	}
	got, err := store.New(db).GetExpenseWithShares(ctx, "expense-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Description != "Primera edición" {
		t.Errorf("Expense after the conflict = %q, want the first edition kept", got.Description)
	}
}

func TestUpdateExpenseThatDoesNotExistFindsNothing(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)

	_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
		e := equalSplit("expense-1")
		e.Version = version
		_, err := q.UpdateExpense(ctx, e, 1)
		return err
	})

	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("UpdateExpense of an unknown Expense: err = %v, want sql.ErrNoRows", err)
	}
}

func TestDeletedExpenseCanBeRestoredWithItsShares(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	q := store.New(db)
	seedGroup(t, db)
	createDinner(t, db, "expense-1")

	deletedAt := write(t, db, func(q *store.Queries, version int64) error {
		_, err := q.DeleteExpense(ctx, store.DeleteExpenseParams{ID: "expense-1", DeletedAt: &now, Version: version})
		return err
	})
	deleted, err := q.GetExpenseWithShares(ctx, "expense-1")
	if err != nil {
		t.Fatal(err)
	}
	if deleted.DeletedAt == nil || deleted.Version != deletedAt {
		t.Errorf("deleted Expense = %+v, want a deletion date and version %d", deleted.Expense, deletedAt)
	}

	restoredAt := write(t, db, func(q *store.Queries, version int64) error {
		_, err := q.RestoreExpense(ctx, store.RestoreExpenseParams{ID: "expense-1", Version: version})
		return err
	})
	restored, err := q.GetExpenseWithShares(ctx, "expense-1")
	if err != nil {
		t.Fatal(err)
	}
	if restored.DeletedAt != nil || restored.Version != restoredAt {
		t.Errorf("restored Expense = %+v, want no deletion date and version %d", restored.Expense, restoredAt)
	}
	if len(restored.PaidShares) != 1 || len(restored.OwedShares) != 2 {
		t.Errorf("restored Expense lost its shares: %+v", restored)
	}
}

func TestRestoreExpenseThatIsNotDeletedChangesNothing(t *testing.T) {
	ctx := context.Background()
	db := openMigrated(t)
	seedGroup(t, db)
	createDinner(t, db, "expense-1")

	_, err := store.Write(ctx, db, func(q *store.Queries, version int64) error {
		_, err := q.RestoreExpense(ctx, store.RestoreExpenseParams{ID: "expense-1", Version: version})
		return err
	})

	if !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("RestoreExpense of a live Expense: err = %v, want sql.ErrNoRows", err)
	}
}
