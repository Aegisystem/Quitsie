package domain_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

// dinner is a 10.50 USD Expense paid by Ana and split equally with Beto.
func dinner() domain.Expense {
	return domain.Expense{
		ID:          "dinner",
		Currency:    "USD",
		Total:       1050,
		SplitMethod: domain.SplitEqual,
		PaidShares:  []domain.PaidShare{{MemberID: "ana", Amount: 1050}},
		OwedShares: []domain.OwedShare{
			{MemberID: "ana", Amount: 525},
			{MemberID: "beto", Amount: 525},
		},
	}
}

func TestRelabelToCurrencyWithSameDecimalsKeepsTheNumbers(t *testing.T) {
	settlement := domain.Settlement{ID: "s1", FromMemberID: "beto", ToMemberID: "ana", Amount: 525, Currency: "USD"}

	expenses, settlements, err := domain.Relabel([]domain.Expense{dinner()}, []domain.Settlement{settlement}, "EUR")
	if err != nil {
		t.Fatal(err)
	}

	wantExpense := dinner()
	wantExpense.Currency = "EUR"
	if !reflect.DeepEqual(expenses, []domain.Expense{wantExpense}) {
		t.Errorf("expenses = %+v, want %+v", expenses, wantExpense)
	}
	wantSettlement := settlement
	wantSettlement.Currency = "EUR"
	if !reflect.DeepEqual(settlements, []domain.Settlement{wantSettlement}) {
		t.Errorf("settlements = %+v, want %+v", settlements, wantSettlement)
	}
}

func TestRelabelToCurrencyWithFewerDecimalsDropsTheZeroDecimals(t *testing.T) {
	taxi := domain.Expense{
		ID:          "taxi",
		Currency:    "USD",
		Total:       1000, // 10.00 USD
		SplitMethod: domain.SplitEqual,
		PaidShares:  []domain.PaidShare{{MemberID: "ana", Amount: 1000}},
		OwedShares: []domain.OwedShare{
			{MemberID: "ana", Amount: 500},
			{MemberID: "beto", Amount: 500},
		},
	}
	settlement := domain.Settlement{ID: "s1", FromMemberID: "beto", ToMemberID: "ana", Amount: 500, Currency: "USD"}

	expenses, settlements, err := domain.Relabel([]domain.Expense{taxi}, []domain.Settlement{settlement}, "JPY")
	if err != nil {
		t.Fatal(err)
	}

	want := domain.Expense{
		ID:          "taxi",
		Currency:    "JPY",
		Total:       10, // 10 JPY
		SplitMethod: domain.SplitEqual,
		PaidShares:  []domain.PaidShare{{MemberID: "ana", Amount: 10}},
		OwedShares: []domain.OwedShare{
			{MemberID: "ana", Amount: 5},
			{MemberID: "beto", Amount: 5},
		},
	}
	if !reflect.DeepEqual(expenses, []domain.Expense{want}) {
		t.Errorf("expenses = %+v, want %+v", expenses, want)
	}
	if got := settlements[0]; got.Amount != 5 || got.Currency != "JPY" {
		t.Errorf("settlement = %d %s, want 5 JPY", got.Amount, got.Currency)
	}
}

func TestRelabelFailsNamingWhatWouldLoseDecimals(t *testing.T) {
	taxi := domain.Expense{
		ID: "taxi", Currency: "USD", Total: 1000, SplitMethod: domain.SplitExact,
		PaidShares: []domain.PaidShare{{MemberID: "ana", Amount: 1000}},
		OwedShares: []domain.OwedShare{{MemberID: "beto", Amount: 1000}},
	}
	// 30.00 USD fits in JPY, but the 9.99 and 10.01 USD OwedShares do not.
	lunch := domain.Expense{
		ID: "lunch", Currency: "USD", Total: 3000, SplitMethod: domain.SplitEqual,
		PaidShares: []domain.PaidShare{{MemberID: "ana", Amount: 3000}},
		OwedShares: []domain.OwedShare{
			{MemberID: "ana", Amount: 999},
			{MemberID: "beto", Amount: 1000},
			{MemberID: "carla", Amount: 1001},
		},
	}
	settlements := []domain.Settlement{
		{ID: "whole", FromMemberID: "beto", ToMemberID: "ana", Amount: 500, Currency: "USD"},
		{ID: "cents", FromMemberID: "beto", ToMemberID: "ana", Amount: 525, Currency: "USD"},
	}

	_, _, err := domain.Relabel([]domain.Expense{dinner(), taxi, lunch}, settlements, "JPY")

	var relabelErr *domain.RelabelError
	if !errors.As(err, &relabelErr) {
		t.Fatalf("err = %v, want a *RelabelError", err)
	}
	if want := []string{"dinner", "lunch"}; !reflect.DeepEqual(relabelErr.ExpenseIDs, want) {
		t.Errorf("ExpenseIDs = %v, want %v", relabelErr.ExpenseIDs, want)
	}
	if want := []string{"cents"}; !reflect.DeepEqual(relabelErr.SettlementIDs, want) {
		t.Errorf("SettlementIDs = %v, want %v", relabelErr.SettlementIDs, want)
	}
}

func TestRelabelToCurrencyWithMoreDecimalsAddsZeroDecimals(t *testing.T) {
	expenses, _, err := domain.Relabel([]domain.Expense{dinner()}, nil, "BHD")
	if err != nil {
		t.Fatal(err)
	}

	got := expenses[0]
	if got.Total != 10500 || got.PaidShares[0].Amount != 10500 || got.OwedShares[0].Amount != 5250 || got.OwedShares[1].Amount != 5250 {
		t.Errorf("relabeled dinner = %+v, want 10.500 BHD split 5.250 + 5.250", got)
	}
}

func TestRelabelFailsWhenAnAmountWouldNotFitInInt64(t *testing.T) {
	huge := domain.Expense{
		ID: "huge", Currency: "JPY", Total: 1_000_000_000_000_000, SplitMethod: domain.SplitExact,
		PaidShares: []domain.PaidShare{{MemberID: "ana", Amount: 1_000_000_000_000_000}},
		OwedShares: []domain.OwedShare{{MemberID: "beto", Amount: 1_000_000_000_000_000}},
	}

	_, _, err := domain.Relabel([]domain.Expense{huge}, nil, "CLF") // 4 decimals

	var relabelErr *domain.RelabelError
	if !errors.As(err, &relabelErr) || !reflect.DeepEqual(relabelErr.ExpenseIDs, []string{"huge"}) {
		t.Errorf("err = %v, want a *RelabelError naming huge", err)
	}
}

func ptr(v int64) *int64 { return &v }

func TestRelabelAlsoRelabelsAdjustmentsButNotPercentagesOrWeights(t *testing.T) {
	// 10.00 USD split equally after giving Ana a +2.00 adjustment.
	adjusted := domain.Expense{
		ID: "adjusted", Currency: "USD", Total: 1000, SplitMethod: domain.SplitAdjustment,
		PaidShares: []domain.PaidShare{{MemberID: "beto", Amount: 1000}},
		OwedShares: []domain.OwedShare{
			{MemberID: "ana", Amount: 600, SplitValue: ptr(200)},
			{MemberID: "beto", Amount: 400, SplitValue: ptr(0)},
		},
	}
	percentages := domain.Expense{
		ID: "percentages", Currency: "USD", Total: 1000, SplitMethod: domain.SplitPercentage,
		PaidShares: []domain.PaidShare{{MemberID: "beto", Amount: 1000}},
		OwedShares: []domain.OwedShare{
			{MemberID: "ana", Amount: 600, SplitValue: ptr(6000)},
			{MemberID: "beto", Amount: 400, SplitValue: ptr(4000)},
		},
	}
	weights := domain.Expense{
		ID: "weights", Currency: "USD", Total: 1000, SplitMethod: domain.SplitWeights,
		PaidShares: []domain.PaidShare{{MemberID: "beto", Amount: 1000}},
		OwedShares: []domain.OwedShare{
			{MemberID: "ana", Amount: 600, SplitValue: ptr(3)},
			{MemberID: "beto", Amount: 400, SplitValue: ptr(2)},
		},
	}

	expenses, _, err := domain.Relabel([]domain.Expense{adjusted, percentages, weights}, nil, "BHD")
	if err != nil {
		t.Fatal(err)
	}

	splitValues := func(e domain.Expense) []int64 {
		var values []int64
		for _, s := range e.OwedShares {
			values = append(values, *s.SplitValue)
		}
		return values
	}
	for i, want := range [][]int64{{2000, 0}, {6000, 4000}, {3, 2}} {
		if got := splitValues(expenses[i]); !reflect.DeepEqual(got, want) {
			t.Errorf("%s split values = %v, want %v", expenses[i].ID, got, want)
		}
	}
	if *adjusted.OwedShares[0].SplitValue != 200 {
		t.Error("Relabel modified its input")
	}
}

func TestRelabelFailsWhenAnAdjustmentWouldLoseDecimals(t *testing.T) {
	// 10.00 USD for Ana alone, re-editable with a +0.50 adjustment.
	adjusted := domain.Expense{
		ID: "adjusted", Currency: "USD", Total: 1000, SplitMethod: domain.SplitAdjustment,
		PaidShares: []domain.PaidShare{{MemberID: "beto", Amount: 1000}},
		OwedShares: []domain.OwedShare{{MemberID: "ana", Amount: 1000, SplitValue: ptr(50)}},
	}

	_, _, err := domain.Relabel([]domain.Expense{adjusted}, nil, "JPY")

	var relabelErr *domain.RelabelError
	if !errors.As(err, &relabelErr) || !reflect.DeepEqual(relabelErr.ExpenseIDs, []string{"adjusted"}) {
		t.Errorf("err = %v, want a *RelabelError naming adjusted", err)
	}
}

func TestRelabelRejectsUnknownCurrencies(t *testing.T) {
	if _, _, err := domain.Relabel([]domain.Expense{dinner()}, nil, "ABC"); !errors.Is(err, domain.ErrUnknownCurrency) {
		t.Errorf("relabel to ABC: err = %v, want ErrUnknownCurrency", err)
	}

	odd := dinner()
	odd.Currency = "ABC"
	if _, _, err := domain.Relabel([]domain.Expense{odd}, nil, "EUR"); !errors.Is(err, domain.ErrUnknownCurrency) {
		t.Errorf("relabel from ABC: err = %v, want ErrUnknownCurrency", err)
	}

	settlement := domain.Settlement{ID: "s1", FromMemberID: "beto", ToMemberID: "ana", Amount: 500, Currency: "ABC"}
	if _, _, err := domain.Relabel(nil, []domain.Settlement{settlement}, "EUR"); !errors.Is(err, domain.ErrUnknownCurrency) {
		t.Errorf("relabel settlement from ABC: err = %v, want ErrUnknownCurrency", err)
	}
}
