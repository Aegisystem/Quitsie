package store_test

import (
	"context"
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
