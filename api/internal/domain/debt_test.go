package domain_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

// expense builds a USD Expense with exact OwedShares; its total is what was paid.
func expense(id string, paid []domain.PaidShare, owed []domain.OwedShare) domain.Expense {
	var total int64
	for _, s := range paid {
		total += s.Amount
	}
	return domain.Expense{
		ID: id, Currency: "USD", Total: total, SplitMethod: domain.SplitExact,
		PaidShares: paid, OwedShares: owed,
	}
}

func TestPairwiseDebtsOfAnExpense(t *testing.T) {
	got := domain.PairwiseDebts([]domain.Expense{dinner()}, nil)

	want := []domain.Debt{{FromMemberID: "beto", ToMemberID: "ana", Currency: "USD", Amount: 525}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PairwiseDebts = %+v, want %+v", got, want)
	}
}

func TestPairwiseDebtsSplitAnExpenseInProportion(t *testing.T) {
	// Ana and Beto paid 6.00 and 4.00; Carla and Dani owe 5.00 each.
	shared := expense("shared",
		[]domain.PaidShare{{MemberID: "ana", Amount: 600}, {MemberID: "beto", Amount: 400}},
		[]domain.OwedShare{{MemberID: "carla", Amount: 500}, {MemberID: "dani", Amount: 500}},
	)

	got := domain.PairwiseDebts([]domain.Expense{shared}, nil)

	want := []domain.Debt{
		{FromMemberID: "carla", ToMemberID: "ana", Currency: "USD", Amount: 300},
		{FromMemberID: "carla", ToMemberID: "beto", Currency: "USD", Amount: 200},
		{FromMemberID: "dani", ToMemberID: "ana", Currency: "USD", Amount: 300},
		{FromMemberID: "dani", ToMemberID: "beto", Currency: "USD", Amount: 200},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PairwiseDebts = %+v, want %+v", got, want)
	}
}

func TestPairwiseDebtsShareOutTheLeftoverDeterministically(t *testing.T) {
	// 1.00 paid half and half by Ana and Beto, owed 0.33 + 0.33 + 0.34: every
	// debtor owes each creditor half of their share, which is not a whole cent.
	paid := []domain.PaidShare{{MemberID: "ana", Amount: 50}, {MemberID: "beto", Amount: 50}}
	owed := []domain.OwedShare{
		{MemberID: "carla", Amount: 33},
		{MemberID: "dani", Amount: 33},
		{MemberID: "eva", Amount: 34},
	}

	got := domain.PairwiseDebts([]domain.Expense{expense("lunch", paid, owed)}, nil)

	share := map[string]int64{"carla": 33, "dani": 33, "eva": 34}
	owes := map[string]int64{}
	isOwed := map[string]int64{}
	for _, d := range got {
		owes[d.FromMemberID] += d.Amount
		isOwed[d.ToMemberID] += d.Amount
		if exact := float64(share[d.FromMemberID]) / 2; float64(d.Amount) < exact-1 || float64(d.Amount) > exact+1 {
			t.Errorf("%s owes %s %d, want within one cent of %.1f", d.FromMemberID, d.ToMemberID, d.Amount, exact)
		}
	}
	if !reflect.DeepEqual(owes, share) {
		t.Errorf("debtors owe %v in total, want %v", owes, share)
	}
	if want := map[string]int64{"ana": 50, "beto": 50}; !reflect.DeepEqual(isOwed, want) {
		t.Errorf("creditors are owed %v in total, want %v", isOwed, want)
	}

	reordered := expense("lunch",
		[]domain.PaidShare{paid[1], paid[0]},
		[]domain.OwedShare{owed[2], owed[0], owed[1]},
	)
	if again := domain.PairwiseDebts([]domain.Expense{reordered}, nil); !reflect.DeepEqual(again, got) {
		t.Errorf("with the shares in another order: %+v, want %+v", again, got)
	}
}

func TestPairwiseDebtsDoNotPanicOnAnInvalidExpense(t *testing.T) {
	invalid := []domain.Expense{
		// Owed shares over what was paid.
		expense("over", []domain.PaidShare{{MemberID: "ana", Amount: 100}}, []domain.OwedShare{{MemberID: "beto", Amount: 300}}),
		// Nobody paid anything.
		expense("unpaid", nil, []domain.OwedShare{{MemberID: "beto", Amount: 300}}),
	}
	for _, e := range invalid {
		domain.PairwiseDebts([]domain.Expense{e}, nil)
	}
}

func TestPairwiseDebtsOffsetEachOther(t *testing.T) {
	anaPaysForBeto := expense("taxi", []domain.PaidShare{{MemberID: "ana", Amount: 1000}}, []domain.OwedShare{{MemberID: "beto", Amount: 1000}})
	betoPaysForAna := expense("coffee", []domain.PaidShare{{MemberID: "beto", Amount: 300}}, []domain.OwedShare{{MemberID: "ana", Amount: 300}})
	betoPaysForCarla := expense("snack", []domain.PaidShare{{MemberID: "beto", Amount: 200}}, []domain.OwedShare{{MemberID: "carla", Amount: 200}})

	got := domain.PairwiseDebts([]domain.Expense{anaPaysForBeto, betoPaysForAna, betoPaysForCarla}, nil)

	// Without debt simplification Carla owes Beto, not Ana.
	want := []domain.Debt{
		{FromMemberID: "beto", ToMemberID: "ana", Currency: "USD", Amount: 700},
		{FromMemberID: "carla", ToMemberID: "beto", Currency: "USD", Amount: 200},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PairwiseDebts = %+v, want %+v", got, want)
	}
}

func TestPairwiseDebtsSubtractSettlements(t *testing.T) {
	tests := []struct {
		name   string
		amount int64
		want   []domain.Debt
	}{
		{"a partial settlement", 200, []domain.Debt{{FromMemberID: "beto", ToMemberID: "ana", Currency: "USD", Amount: 325}}},
		{"a full settlement", 525, nil},
		{"paying back too much", 600, []domain.Debt{{FromMemberID: "ana", ToMemberID: "beto", Currency: "USD", Amount: 75}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settlement := payback()
			settlement.Amount = tt.amount
			if got := domain.PairwiseDebts([]domain.Expense{dinner()}, []domain.Settlement{settlement}); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PairwiseDebts = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestPairwiseDebtsIgnoreDeletedExpensesAndSettlements(t *testing.T) {
	deletedAt := time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)
	deletedDinner := dinner()
	deletedDinner.DeletedAt = &deletedAt
	deletedPayback := payback()
	deletedPayback.DeletedAt = &deletedAt

	if got := domain.PairwiseDebts([]domain.Expense{deletedDinner}, nil); len(got) != 0 {
		t.Errorf("with a deleted expense: PairwiseDebts = %+v, want none", got)
	}
	want := []domain.Debt{{FromMemberID: "beto", ToMemberID: "ana", Currency: "USD", Amount: 525}}
	if got := domain.PairwiseDebts([]domain.Expense{dinner()}, []domain.Settlement{deletedPayback}); !reflect.DeepEqual(got, want) {
		t.Errorf("with a deleted settlement: PairwiseDebts = %+v, want %+v", got, want)
	}
}

func TestPairwiseDebtsHandleAmountsWhoseProductOverflowsInt64(t *testing.T) {
	const big = 4_000_000_000_000_000_000 // big * big does not fit in an int64
	huge := expense("huge",
		[]domain.PaidShare{{MemberID: "ana", Amount: big}, {MemberID: "beto", Amount: big}},
		[]domain.OwedShare{{MemberID: "carla", Amount: big}, {MemberID: "dani", Amount: big}},
	)

	got := domain.PairwiseDebts([]domain.Expense{huge}, nil)

	want := []domain.Debt{
		{FromMemberID: "carla", ToMemberID: "ana", Currency: "USD", Amount: big / 2},
		{FromMemberID: "carla", ToMemberID: "beto", Currency: "USD", Amount: big / 2},
		{FromMemberID: "dani", ToMemberID: "ana", Currency: "USD", Amount: big / 2},
		{FromMemberID: "dani", ToMemberID: "beto", Currency: "USD", Amount: big / 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PairwiseDebts = %+v, want %+v", got, want)
	}
}

func TestPairwiseDebtsKeepEachCurrencyApart(t *testing.T) {
	sushi := domain.Expense{
		ID: "sushi", Currency: "JPY", Total: 3000, SplitMethod: domain.SplitExact,
		PaidShares: []domain.PaidShare{{MemberID: "beto", Amount: 3000}},
		OwedShares: []domain.OwedShare{{MemberID: "ana", Amount: 3000}},
	}

	got := domain.PairwiseDebts([]domain.Expense{dinner(), sushi}, nil)

	want := []domain.Debt{
		{FromMemberID: "ana", ToMemberID: "beto", Currency: "JPY", Amount: 3000},
		{FromMemberID: "beto", ToMemberID: "ana", Currency: "USD", Amount: 525},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("PairwiseDebts = %+v, want %+v", got, want)
	}
}
