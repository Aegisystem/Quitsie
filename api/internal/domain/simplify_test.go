package domain_test

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

func TestSimplifyDebtsBetweenTwoMembers(t *testing.T) {
	balances := []domain.Balance{
		{MemberID: "ana", Currency: "USD", Amount: 525},
		{MemberID: "beto", Currency: "USD", Amount: -525},
	}

	got := domain.SimplifyDebts(balances)

	want := []domain.Debt{{FromMemberID: "beto", ToMemberID: "ana", Currency: "USD", Amount: 525}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SimplifyDebts = %+v, want %+v", got, want)
	}
}

func TestSimplifyDebtsSkipsTheMiddleOfAChain(t *testing.T) {
	// Ana owes Beto 10.00 and Beto owes Carla 10.00, so Beto's Balance is zero.
	balances := []domain.Balance{
		{MemberID: "ana", Currency: "USD", Amount: -1000},
		{MemberID: "carla", Currency: "USD", Amount: 1000},
	}

	got := domain.SimplifyDebts(balances)

	want := []domain.Debt{{FromMemberID: "ana", ToMemberID: "carla", Currency: "USD", Amount: 1000}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SimplifyDebts = %+v, want %+v", got, want)
	}
}

func TestSimplifyDebtsPaysTheLargestDebtorToTheLargestCreditorFirst(t *testing.T) {
	balances := []domain.Balance{
		{MemberID: "ana", Currency: "USD", Amount: 50},
		{MemberID: "beto", Currency: "USD", Amount: 30},
		{MemberID: "carla", Currency: "USD", Amount: -40},
		{MemberID: "dani", Currency: "USD", Amount: -25},
		{MemberID: "eva", Currency: "USD", Amount: -15},
	}

	got := domain.SimplifyDebts(balances)

	// Carla pays Ana 40; Dani pays Beto 25; Eva pays Ana the last 10 and Beto the last 5.
	want := []domain.Debt{
		{FromMemberID: "carla", ToMemberID: "ana", Currency: "USD", Amount: 40},
		{FromMemberID: "dani", ToMemberID: "beto", Currency: "USD", Amount: 25},
		{FromMemberID: "eva", ToMemberID: "ana", Currency: "USD", Amount: 10},
		{FromMemberID: "eva", ToMemberID: "beto", Currency: "USD", Amount: 5},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SimplifyDebts = %+v, want %+v", got, want)
	}
}

func TestSimplifyDebtsBreaksTiesByMember(t *testing.T) {
	balances := []domain.Balance{
		{MemberID: "dani", Currency: "USD", Amount: -10},
		{MemberID: "beto", Currency: "USD", Amount: 10},
		{MemberID: "carla", Currency: "USD", Amount: -10},
		{MemberID: "ana", Currency: "USD", Amount: 10},
	}

	got := domain.SimplifyDebts(balances)

	want := []domain.Debt{
		{FromMemberID: "carla", ToMemberID: "ana", Currency: "USD", Amount: 10},
		{FromMemberID: "dani", ToMemberID: "beto", Currency: "USD", Amount: 10},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SimplifyDebts = %+v, want %+v", got, want)
	}
}

func TestSimplifyDebtsKeepsEachCurrencyApart(t *testing.T) {
	balances := []domain.Balance{
		{MemberID: "ana", Currency: "JPY", Amount: -3000},
		{MemberID: "beto", Currency: "JPY", Amount: 3000},
		{MemberID: "ana", Currency: "USD", Amount: 525},
		{MemberID: "beto", Currency: "USD", Amount: -525},
	}

	got := domain.SimplifyDebts(balances)

	want := []domain.Debt{
		{FromMemberID: "ana", ToMemberID: "beto", Currency: "JPY", Amount: 3000},
		{FromMemberID: "beto", ToMemberID: "ana", Currency: "USD", Amount: 525},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("SimplifyDebts = %+v, want %+v", got, want)
	}
}

func TestSimplifyDebtsSettlesEveryBalanceInAtMostNMinusOnePayments(t *testing.T) {
	rng := rand.New(rand.NewPCG(2026, 10))
	for round := range 500 {
		balances := randomBalances(rng)

		got := domain.SimplifyDebts(balances)

		if n := len(balances); len(got) > max(n-1, 0) {
			t.Fatalf("round %d: %d payments for %d Members with a Balance, want at most %d", round, len(got), n, max(n-1, 0))
		}
		left := map[string]int64{}
		for _, b := range balances {
			left[b.MemberID] = b.Amount
		}
		for _, d := range got {
			if d.Amount <= 0 {
				t.Fatalf("round %d: payment %+v is not positive", round, d)
			}
			left[d.FromMemberID] += d.Amount
			left[d.ToMemberID] -= d.Amount
		}
		for member, amount := range left {
			if amount != 0 {
				t.Fatalf("round %d: %s is left with %d after the payments, want 0", round, member, amount)
			}
		}

		shuffled := append([]domain.Balance(nil), balances...)
		rng.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
		if again := domain.SimplifyDebts(shuffled); !reflect.DeepEqual(again, got) {
			t.Fatalf("round %d: with the Balances in another order got %+v, want %+v", round, again, got)
		}
	}
}

// randomBalances makes the non-zero USD Balances of 2 to 12 Members, adding up to zero.
func randomBalances(rng *rand.Rand) []domain.Balance {
	n := 2 + rng.IntN(11)
	amounts := make([]int64, n)
	var sum int64
	for i := range n - 1 {
		amounts[i] = rng.Int64N(20001) - 10000
		sum += amounts[i]
	}
	amounts[n-1] = -sum

	var balances []domain.Balance
	for i, amount := range amounts {
		if amount != 0 {
			balances = append(balances, domain.Balance{MemberID: fmt.Sprintf("m%02d", i), Currency: "USD", Amount: amount})
		}
	}
	return balances
}
