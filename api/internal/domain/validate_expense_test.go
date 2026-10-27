package domain_test

import (
	"errors"
	"math"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

// trip is a Group in USD where Dani already left.
func trip() domain.Ledger {
	return domain.Ledger{
		Kind:     domain.LedgerGroup,
		Currency: "USD",
		Members: []domain.Member{
			{ID: "ana", Status: domain.MemberActive},
			{ID: "beto", Status: domain.MemberActive},
			{ID: "carla", Status: domain.MemberActive},
			{ID: "dani", Status: domain.MemberFormer},
		},
	}
}

func TestValidateExpenseAcceptsAValidGroupExpense(t *testing.T) {
	if err := domain.ValidateExpense(trip(), dinner()); err != nil {
		t.Errorf("ValidateExpense(dinner) = %v, want nil", err)
	}
}

func TestValidateExpenseRejects(t *testing.T) {
	tests := []struct {
		name   string
		change func(e *domain.Expense)
		want   error
	}{
		{"a zero total", func(e *domain.Expense) { e.Total = 0 }, domain.ErrNonPositiveAmount},
		{"a negative total", func(e *domain.Expense) { e.Total = -1050 }, domain.ErrNonPositiveAmount},
		{"no paid shares", func(e *domain.Expense) { e.PaidShares = nil }, domain.ErrMissingShares},
		{"no owed shares", func(e *domain.Expense) { e.OwedShares = nil }, domain.ErrMissingShares},
		{"paid shares under the total", func(e *domain.Expense) { e.PaidShares[0].Amount = 1049 }, domain.ErrSharesDoNotAddUp},
		{"paid shares over the total", func(e *domain.Expense) {
			e.PaidShares = append(e.PaidShares, domain.PaidShare{MemberID: "beto", Amount: 1})
		}, domain.ErrSharesDoNotAddUp},
		{"owed shares under the total", func(e *domain.Expense) { e.OwedShares[1].Amount = 524 }, domain.ErrSharesDoNotAddUp},
		{"owed shares over the total", func(e *domain.Expense) { e.OwedShares[1].Amount = 526 }, domain.ErrSharesDoNotAddUp},
		{"a negative paid share that still adds up", func(e *domain.Expense) {
			e.PaidShares = []domain.PaidShare{{MemberID: "ana", Amount: 1100}, {MemberID: "beto", Amount: -50}}
		}, domain.ErrNonPositiveAmount},
		{"a zero owed share", func(e *domain.Expense) {
			e.OwedShares = append(e.OwedShares, domain.OwedShare{MemberID: "carla", Amount: 0})
		}, domain.ErrNonPositiveAmount},
		{"paid shares that add up only by overflowing", func(e *domain.Expense) {
			e.PaidShares = []domain.PaidShare{
				{MemberID: "ana", Amount: math.MaxInt64},
				{MemberID: "beto", Amount: math.MaxInt64},
				{MemberID: "carla", Amount: 1052}, // MaxInt64 + MaxInt64 + 1052 wraps to 1050
			}
		}, domain.ErrSharesDoNotAddUp},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := dinner()
			tt.change(&e)
			if err := domain.ValidateExpense(trip(), e); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
