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
		{"a zero paid share", func(e *domain.Expense) {
			e.PaidShares = append(e.PaidShares, domain.PaidShare{MemberID: "beto", Amount: 0})
		}, domain.ErrNonPositiveAmount},
		{"a negative owed share that still adds up", func(e *domain.Expense) {
			e.OwedShares = []domain.OwedShare{{MemberID: "ana", Amount: 1100}, {MemberID: "beto", Amount: -50}}
		}, domain.ErrNonPositiveAmount},
		{"paid shares that add up only by overflowing", func(e *domain.Expense) {
			e.PaidShares = []domain.PaidShare{
				{MemberID: "ana", Amount: math.MaxInt64},
				{MemberID: "beto", Amount: math.MaxInt64},
				{MemberID: "carla", Amount: 1052}, // MaxInt64 + MaxInt64 + 1052 wraps to 1050
			}
		}, domain.ErrSharesDoNotAddUp},
		{"the same Participant paying twice", func(e *domain.Expense) {
			e.PaidShares = []domain.PaidShare{{MemberID: "ana", Amount: 1000}, {MemberID: "ana", Amount: 50}}
		}, domain.ErrDuplicateParticipant},
		{"the same Participant owing twice", func(e *domain.Expense) { e.OwedShares[1].MemberID = "ana" }, domain.ErrDuplicateParticipant},
		{"a payer who is not in the Group", func(e *domain.Expense) { e.PaidShares[0].MemberID = "eva" }, domain.ErrNotActiveMember},
		{"a debtor who is not in the Group", func(e *domain.Expense) { e.OwedShares[1].MemberID = "eva" }, domain.ErrNotActiveMember},
		{"a former member", func(e *domain.Expense) { e.OwedShares[1].MemberID = "dani" }, domain.ErrNotActiveMember},
		{"a currency other than the Group currency", func(e *domain.Expense) { e.Currency = "EUR" }, domain.ErrNotGroupCurrency},
		{"an unknown currency", func(e *domain.Expense) { e.Currency = "ABC" }, domain.ErrUnknownCurrency},
		{"an unknown split method", func(e *domain.Expense) { e.SplitMethod = "shares" }, domain.ErrUnknownSplitMethod},
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

func TestValidateExpenseAcceptsAnOwedShareRoundedToZero(t *testing.T) {
	// One cent split equally between Ana and Beto: Beto's OwedShare rounds to
	// zero, and it is kept so the split survives re-editing.
	cent := domain.Expense{
		ID: "gum", Currency: "USD", Total: 1, SplitMethod: domain.SplitEqual,
		PaidShares: []domain.PaidShare{{MemberID: "ana", Amount: 1}},
		OwedShares: []domain.OwedShare{{MemberID: "ana", Amount: 1}, {MemberID: "beto", Amount: 0}},
	}
	if err := domain.ValidateExpense(trip(), cent); err != nil {
		t.Errorf("ValidateExpense(1 cent split in two) = %v, want nil", err)
	}
}

func TestValidateExpenseCapsTheTotalAtTenToTheFifteen(t *testing.T) {
	huge := func(total int64) domain.Expense {
		return domain.Expense{
			ID: "yacht", Currency: "USD", Total: total, SplitMethod: domain.SplitExact,
			PaidShares: []domain.PaidShare{{MemberID: "ana", Amount: total}},
			OwedShares: []domain.OwedShare{{MemberID: "beto", Amount: total}},
		}
	}

	if err := domain.ValidateExpense(trip(), huge(1_000_000_000_000_000)); err != nil {
		t.Errorf("total of 10^15: err = %v, want nil", err)
	}
	if err := domain.ValidateExpense(trip(), huge(1_000_000_000_000_001)); !errors.Is(err, domain.ErrAmountTooLarge) {
		t.Errorf("total over 10^15: err = %v, want ErrAmountTooLarge", err)
	}
}

// friendship is the ledger of Ana and her Friend Beto.
func friendship() domain.Ledger {
	return domain.Ledger{
		Kind: domain.LedgerFriendship,
		Members: []domain.Member{
			{ID: "ana", Status: domain.MemberActive},
			{ID: "beto", Status: domain.MemberActive},
		},
	}
}

func TestValidateExpenseAcceptsAnyCurrencyInAFriendship(t *testing.T) {
	for _, c := range []domain.Currency{"USD", "JPY", "KWD"} {
		e := dinner()
		e.Currency = c
		if err := domain.ValidateExpense(friendship(), e); err != nil {
			t.Errorf("ValidateExpense(dinner in %s) = %v, want nil", c, err)
		}
	}
}

func TestValidateExpenseRejectsAFriendshipWithoutExactlyTwoMembers(t *testing.T) {
	crowded := friendship()
	crowded.Members = append(crowded.Members, domain.Member{ID: "carla", Status: domain.MemberActive})
	alone := friendship()
	alone.Members = alone.Members[:1]

	for name, ledger := range map[string]domain.Ledger{"three members": crowded, "one member": alone} {
		if err := domain.ValidateExpense(ledger, dinner()); !errors.Is(err, domain.ErrFriendshipNotTwoMembers) {
			t.Errorf("%s: err = %v, want ErrFriendshipNotTwoMembers", name, err)
		}
	}
}

func TestValidateExpenseRejectsAnUnknownLedgerKind(t *testing.T) {
	ledger := trip()
	ledger.Kind = "club"
	if err := domain.ValidateExpense(ledger, dinner()); !errors.Is(err, domain.ErrUnknownLedgerKind) {
		t.Errorf("err = %v, want ErrUnknownLedgerKind", err)
	}
}

func TestValidateExpenseChecksTheSplitValues(t *testing.T) {
	tests := []struct {
		name   string
		method domain.SplitMethod
		values []*int64 // one per OwedShare of dinner: Ana 5.25, Beto 5.25
		want   error
	}{
		{"percentages adding up to 100%", domain.SplitPercentage, []*int64{ptr(5000), ptr(5000)}, nil},
		{"percentages with two decimals", domain.SplitPercentage, []*int64{ptr(4999), ptr(5001)}, nil},
		{"percentages under 100%", domain.SplitPercentage, []*int64{ptr(5000), ptr(4999)}, domain.ErrInvalidSplitValues},
		{"percentages over 100%", domain.SplitPercentage, []*int64{ptr(5000), ptr(5001)}, domain.ErrInvalidSplitValues},
		{"a missing percentage", domain.SplitPercentage, []*int64{ptr(10000), nil}, domain.ErrInvalidSplitValues},
		{"a negative percentage", domain.SplitPercentage, []*int64{ptr(12000), ptr(-2000)}, domain.ErrInvalidSplitValues},
		{"a zero percentage", domain.SplitPercentage, []*int64{ptr(10000), ptr(0)}, domain.ErrInvalidSplitValues},
		{"positive weights", domain.SplitWeights, []*int64{ptr(1), ptr(1)}, nil},
		{"a missing weight", domain.SplitWeights, []*int64{ptr(1), nil}, domain.ErrInvalidSplitValues},
		{"a zero weight", domain.SplitWeights, []*int64{ptr(1), ptr(0)}, domain.ErrInvalidSplitValues},
		{"a negative weight", domain.SplitWeights, []*int64{ptr(2), ptr(-1)}, domain.ErrInvalidSplitValues},
		{"adjustments of any sign", domain.SplitAdjustment, []*int64{ptr(-25), ptr(0)}, nil},
		{"an adjustment over 10^15", domain.SplitAdjustment, []*int64{ptr(domain.MaxAmount + 1), ptr(0)}, domain.ErrAmountTooLarge},
		{"an adjustment under -10^15", domain.SplitAdjustment, []*int64{ptr(-domain.MaxAmount - 1), ptr(0)}, domain.ErrAmountTooLarge},
		{"a missing adjustment", domain.SplitAdjustment, []*int64{ptr(25), nil}, domain.ErrInvalidSplitValues},
		{"equal parts without values", domain.SplitEqual, []*int64{nil, nil}, nil},
		{"equal parts with a value", domain.SplitEqual, []*int64{nil, ptr(1)}, domain.ErrInvalidSplitValues},
		{"exact amounts without values", domain.SplitExact, []*int64{nil, nil}, nil},
		{"exact amounts with a value", domain.SplitExact, []*int64{ptr(525), nil}, domain.ErrInvalidSplitValues},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := dinner()
			e.SplitMethod = tt.method
			for i, v := range tt.values {
				e.OwedShares[i].SplitValue = v
			}
			if err := domain.ValidateExpense(trip(), e); !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
