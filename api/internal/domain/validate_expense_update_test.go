package domain_test

import (
	"errors"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

// groceries is an Expense of the trip from before Dani left: Ana and Dani
// paid, and the three of them owe 10 USD each.
func groceries() domain.Expense {
	return domain.Expense{
		ID: "groceries", Currency: "USD", Total: 3000, SplitMethod: domain.SplitExact,
		PaidShares: []domain.PaidShare{{MemberID: "ana", Amount: 1500}, {MemberID: "dani", Amount: 1500}},
		OwedShares: []domain.OwedShare{
			{MemberID: "ana", Amount: 1000},
			{MemberID: "beto", Amount: 1000},
			{MemberID: "dani", Amount: 1000},
		},
	}
}

func TestValidateExpenseUpdateKeepsAFormerMemberWhoseSharesDoNotChange(t *testing.T) {
	edited := groceries()
	edited.OwedShares[0].Amount, edited.OwedShares[1].Amount = 1200, 800

	if err := domain.ValidateExpenseUpdate(trip(), groceries(), edited); err != nil {
		t.Errorf("ValidateExpenseUpdate = %v, want nil", err)
	}
}

func TestValidateExpenseUpdateRejects(t *testing.T) {
	tests := []struct {
		name   string
		change func(e *domain.Expense)
		want   error
	}{
		{"a former member's OwedShare changing", func(e *domain.Expense) {
			e.OwedShares[1].Amount, e.OwedShares[2].Amount = 1500, 500
		}, domain.ErrFormerMemberChanged},
		{"a former member's PaidShare changing", func(e *domain.Expense) {
			e.PaidShares[0].Amount, e.PaidShares[1].Amount = 2000, 1000
		}, domain.ErrFormerMemberChanged},
		{"a former member leaving the Expense", func(e *domain.Expense) {
			e.PaidShares = []domain.PaidShare{{MemberID: "ana", Amount: 3000}}
			e.OwedShares = []domain.OwedShare{{MemberID: "ana", Amount: 1500}, {MemberID: "beto", Amount: 1500}}
		}, domain.ErrFormerMemberChanged},
		{"shares that do not add up", func(e *domain.Expense) { e.OwedShares[0].Amount = 999 }, domain.ErrSharesDoNotAddUp},
		{"someone who is not in the Group", func(e *domain.Expense) { e.OwedShares[1].MemberID = "eva" }, domain.ErrNotActiveMember},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edited := groceries()
			tt.change(&edited)
			if err := domain.ValidateExpenseUpdate(trip(), groceries(), edited); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateExpenseUpdateRejectsAddingAFormerMember(t *testing.T) {
	edited := dinner()
	edited.OwedShares = append(edited.OwedShares, domain.OwedShare{MemberID: "dani", Amount: 0})

	if err := domain.ValidateExpenseUpdate(trip(), dinner(), edited); !errors.Is(err, domain.ErrFormerMemberChanged) {
		t.Errorf("err = %v, want ErrFormerMemberChanged", err)
	}
}
