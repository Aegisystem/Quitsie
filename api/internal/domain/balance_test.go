package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

func TestBalancesOfAnExpense(t *testing.T) {
	got := domain.Balances([]domain.Expense{dinner()}, nil)

	want := []domain.Balance{
		{MemberID: "ana", Currency: "USD", Amount: 525},
		{MemberID: "beto", Currency: "USD", Amount: -525},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Balances = %+v, want %+v", got, want)
	}
}

func TestBalancesCountSettlements(t *testing.T) {
	partial := payback()
	partial.Amount = 200

	got := domain.Balances([]domain.Expense{dinner()}, []domain.Settlement{partial})

	want := []domain.Balance{
		{MemberID: "ana", Currency: "USD", Amount: 325},
		{MemberID: "beto", Currency: "USD", Amount: -325},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Balances = %+v, want %+v", got, want)
	}
}

func TestBalancesLeaveOutSettledMembers(t *testing.T) {
	got := domain.Balances([]domain.Expense{dinner()}, []domain.Settlement{payback()})

	if len(got) != 0 {
		t.Errorf("Balances = %+v, want none", got)
	}
}

func TestBalancesIgnoreDeletedExpensesAndSettlements(t *testing.T) {
	deletedAt := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	deletedDinner := dinner()
	deletedDinner.DeletedAt = &deletedAt
	deletedPayback := payback()
	deletedPayback.DeletedAt = &deletedAt

	if got := domain.Balances([]domain.Expense{deletedDinner}, nil); len(got) != 0 {
		t.Errorf("with a deleted expense: Balances = %+v, want none", got)
	}
	want := []domain.Balance{
		{MemberID: "ana", Currency: "USD", Amount: 525},
		{MemberID: "beto", Currency: "USD", Amount: -525},
	}
	if got := domain.Balances([]domain.Expense{dinner()}, []domain.Settlement{deletedPayback}); !reflect.DeepEqual(got, want) {
		t.Errorf("with a deleted settlement: Balances = %+v, want %+v", got, want)
	}
}

func TestBalancesKeepEachCurrencyApart(t *testing.T) {
	sushi := domain.Expense{
		ID: "sushi", Currency: "JPY", Total: 3000, SplitMethod: domain.SplitExact,
		PaidShares: []domain.PaidShare{{MemberID: "beto", Amount: 3000}},
		OwedShares: []domain.OwedShare{{MemberID: "ana", Amount: 3000}},
	}

	got := domain.Balances([]domain.Expense{sushi, dinner()}, nil)

	want := []domain.Balance{
		{MemberID: "ana", Currency: "JPY", Amount: -3000},
		{MemberID: "beto", Currency: "JPY", Amount: 3000},
		{MemberID: "ana", Currency: "USD", Amount: 525},
		{MemberID: "beto", Currency: "USD", Amount: -525},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Balances = %+v, want %+v", got, want)
	}
}
