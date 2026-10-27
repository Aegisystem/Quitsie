package domain_test

import (
	"errors"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

// payback is Beto paying Ana back 5.25 USD.
func payback() domain.Settlement {
	return domain.Settlement{ID: "payback", FromMemberID: "beto", ToMemberID: "ana", Amount: 525, Currency: "USD"}
}

func TestValidateSettlementAcceptsAValidSettlement(t *testing.T) {
	if err := domain.ValidateSettlement(trip(), payback()); err != nil {
		t.Errorf("ValidateSettlement(payback) = %v, want nil", err)
	}
}

func TestValidateSettlementRejects(t *testing.T) {
	tests := []struct {
		name   string
		change func(s *domain.Settlement)
		want   error
	}{
		{"a zero amount", func(s *domain.Settlement) { s.Amount = 0 }, domain.ErrNonPositiveAmount},
		{"a negative amount", func(s *domain.Settlement) { s.Amount = -525 }, domain.ErrNonPositiveAmount},
		{"an amount over 10^15", func(s *domain.Settlement) { s.Amount = domain.MaxAmount + 1 }, domain.ErrAmountTooLarge},
		{"paying oneself", func(s *domain.Settlement) { s.ToMemberID = "beto" }, domain.ErrSameMember},
		{"a payer who is not in the Group", func(s *domain.Settlement) { s.FromMemberID = "eva" }, domain.ErrNotActiveMember},
		{"a payee who is not in the Group", func(s *domain.Settlement) { s.ToMemberID = "eva" }, domain.ErrNotActiveMember},
		{"a former member", func(s *domain.Settlement) { s.ToMemberID = "dani" }, domain.ErrNotActiveMember},
		{"a currency other than the Group currency", func(s *domain.Settlement) { s.Currency = "EUR" }, domain.ErrNotGroupCurrency},
		{"an unknown currency", func(s *domain.Settlement) { s.Currency = "ABC" }, domain.ErrUnknownCurrency},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := payback()
			tt.change(&s)
			if err := domain.ValidateSettlement(trip(), s); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateSettlementInAFriendship(t *testing.T) {
	inYen := payback()
	inYen.Currency = "JPY"
	if err := domain.ValidateSettlement(friendship(), inYen); err != nil {
		t.Errorf("settlement in JPY: err = %v, want nil", err)
	}

	crowded := friendship()
	crowded.Members = append(crowded.Members, domain.Member{ID: "carla", Status: domain.MemberActive})
	if err := domain.ValidateSettlement(crowded, payback()); !errors.Is(err, domain.ErrFriendshipNotTwoMembers) {
		t.Errorf("three members: err = %v, want ErrFriendshipNotTwoMembers", err)
	}
}
