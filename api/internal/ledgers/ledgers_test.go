package ledgers_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
	"github.com/Aegisystem/Quitsie/api/internal/ledgers"
	"github.com/Aegisystem/Quitsie/api/internal/store"
)

func TestLedgerMapsKindCurrencyAndMembers(t *testing.T) {
	eur := "EUR"
	got := ledgers.Ledger(
		store.Ledger{Kind: "group", Currency: &eur},
		[]store.Member{{ID: "a", Status: "active"}, {ID: "b", Status: "former"}},
	)

	want := domain.Ledger{
		Kind:     domain.LedgerGroup,
		Currency: "EUR",
		Members:  []domain.Member{{ID: "a", Status: domain.MemberActive}, {ID: "b", Status: domain.MemberFormer}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Ledger = %+v, want %+v", got, want)
	}
}

func TestFriendshipLedgerHasNoCurrency(t *testing.T) {
	got := ledgers.Ledger(store.Ledger{Kind: "friendship"}, nil)

	if got.Kind != domain.LedgerFriendship || got.Currency != "" {
		t.Errorf("Ledger = %+v, want a friendship without currency", got)
	}
}

func TestExpenseMapsSharesAndDeletion(t *testing.T) {
	deleted := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	weight := int64(2)
	got := ledgers.Expense(store.ExpenseWithShares{
		Expense:    store.Expense{ID: "e", Currency: "COP", Total: 300, SplitMethod: "weights", DeletedAt: &deleted},
		PaidShares: []store.PaidShare{{ExpenseID: "e", MemberID: "a", Amount: 300}},
		OwedShares: []store.OwedShare{{ExpenseID: "e", MemberID: "a", Amount: 200, SplitValue: &weight}},
	})

	want := domain.Expense{
		ID:          "e",
		Currency:    "COP",
		Total:       300,
		SplitMethod: domain.SplitWeights,
		PaidShares:  []domain.PaidShare{{MemberID: "a", Amount: 300}},
		OwedShares:  []domain.OwedShare{{MemberID: "a", Amount: 200, SplitValue: &weight}},
		DeletedAt:   &deleted,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Expense = %+v, want %+v", got, want)
	}
}

func TestSettlementMapsBothMembers(t *testing.T) {
	got := ledgers.Settlement(store.Settlement{ID: "s", FromMemberID: "a", ToMemberID: "b", Amount: 50, Currency: "USD"})

	want := domain.Settlement{ID: "s", FromMemberID: "a", ToMemberID: "b", Amount: 50, Currency: "USD"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Settlement = %+v, want %+v", got, want)
	}
}
