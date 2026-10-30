package domain_test

import (
	"errors"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

func TestValidateExpenseDeletion(t *testing.T) {
	zeroShare := dinner()
	zeroShare.OwedShares = append(zeroShare.OwedShares, domain.OwedShare{MemberID: "dani", Amount: 0})
	tests := []struct {
		name    string
		expense domain.Expense
		want    error
	}{
		{"only active Members", dinner(), nil},
		{"a former member with shares", groceries(), domain.ErrFormerMemberChanged},
		{"a former member whose OwedShare rounded to zero", zeroShare, domain.ErrFormerMemberChanged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := domain.ValidateExpenseDeletion(trip(), tt.expense); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateSettlementDeletion(t *testing.T) {
	from, to := payback(), payback()
	from.FromMemberID = "dani"
	to.ToMemberID = "dani"
	tests := []struct {
		name       string
		settlement domain.Settlement
		want       error
	}{
		{"only active Members", payback(), nil},
		{"paid by a former member", from, domain.ErrFormerMemberChanged},
		{"paid to a former member", to, domain.ErrFormerMemberChanged},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := domain.ValidateSettlementDeletion(trip(), tt.settlement); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
