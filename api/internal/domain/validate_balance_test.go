package domain_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Aegisystem/Quitsie/api/internal/domain"
)

// lent is USD Expenses of at most MaxAmount in which payer pays for debtor,
// adding up to amount: they move the payer's Balance up by amount and the
// debtor's down by as much.
func lent(payer, debtor string, amount int64) []domain.Expense {
	var out []domain.Expense
	for i := 0; amount > 0; i++ {
		a := min(amount, domain.MaxAmount)
		out = append(out, domain.Expense{
			ID: fmt.Sprintf("%s-%s-%d", payer, debtor, i), Currency: "USD", Total: a, SplitMethod: domain.SplitExact,
			PaidShares: []domain.PaidShare{{MemberID: payer, Amount: a}},
			OwedShares: []domain.OwedShare{{MemberID: debtor, Amount: a}},
		})
		amount -= a
	}
	return out
}

// lentMany is n Expenses in which payer pays MaxAmount for debtor, which may
// add up to more than an int64 holds.
func lentMany(n int, payer, debtor string) []domain.Expense {
	out := make([]domain.Expense, n)
	for i := range out {
		out[i] = lent(payer, debtor, domain.MaxAmount)[0]
	}
	return out
}

// tripWith is the trip with records already in it.
func tripWith(expenses []domain.Expense, settlements []domain.Settlement) domain.Ledger {
	ledger := trip()
	ledger.AddUp(expenses, settlements)
	return ledger
}

func TestValidateExpenseAcceptsBalancesUpToTheLimitEitherWay(t *testing.T) {
	// Ana is owed and Beto owes MaxBalance - MaxAmount; one more Expense of
	// MaxAmount takes them exactly to the limit.
	ledger := tripWith(lent("ana", "beto", domain.MaxBalance-domain.MaxAmount), nil)

	if err := domain.ValidateExpense(ledger, lent("ana", "beto", domain.MaxAmount)[0]); err != nil {
		t.Errorf("ValidateExpense = %v, want nil: Ana reaches MaxBalance and Beto -MaxBalance", err)
	}
}

func TestValidateExpenseRejectsABalanceOneOverTheLimit(t *testing.T) {
	tests := []struct {
		name   string
		before []domain.Expense
		over   string
	}{
		{"in favor", lent("ana", "carla", domain.MaxBalance-domain.MaxAmount+1), "ana"},
		{"against", lent("carla", "beto", domain.MaxBalance-domain.MaxAmount+1), "beto"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger := tripWith(tt.before, nil)

			err := domain.ValidateExpense(ledger, lent("ana", "beto", domain.MaxAmount)[0])

			if !errors.Is(err, domain.ErrBalanceTooLarge) {
				t.Fatalf("ValidateExpense = %v, want ErrBalanceTooLarge", err)
			}
			if want := fmt.Sprintf("%q", tt.over); !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %s", err, want)
			}
		})
	}
}

func TestValidateExpenseAddsUpBalancesExactly(t *testing.T) {
	t.Run("not fooled by an int64 that wraps around", func(t *testing.T) {
		// Beto lent Ana 18,447 × 10^15: an int64 wraps that to -2.6 × 10^14,
		// which looks well within the limit.
		ledger := tripWith(lentMany(18_447, "beto", "ana"), nil)

		err := domain.ValidateExpense(ledger, lent("ana", "carla", 100)[0])

		if !errors.Is(err, domain.ErrBalanceTooLarge) {
			t.Errorf("ValidateExpense = %v, want ErrBalanceTooLarge", err)
		}
	})
	t.Run("not misled by sums that go past an int64 on the way", func(t *testing.T) {
		// 10^19 one way and then back: every Balance ends at zero.
		ledger := tripWith(append(lentMany(10_000, "ana", "beto"), lentMany(10_000, "beto", "ana")...), nil)

		if err := domain.ValidateExpense(ledger, lent("ana", "beto", domain.MaxAmount)[0]); err != nil {
			t.Errorf("ValidateExpense = %v, want nil", err)
		}
	})
}

func TestValidatingABatchCountsTheRecordsStoredBeforeIt(t *testing.T) {
	// Room for two and a half Expenses of MaxAmount, as the pusher sees it.
	ledger := tripWith(lent("ana", "beto", domain.MaxBalance-5*domain.MaxAmount/2), nil)
	var errs []error
	for _, e := range lentMany(3, "ana", "beto") {
		err := domain.ValidateExpense(ledger, e)
		if err == nil {
			ledger.AddUp([]domain.Expense{e}, nil)
		}
		errs = append(errs, err)
	}

	if errs[0] != nil || errs[1] != nil || !errors.Is(errs[2], domain.ErrBalanceTooLarge) {
		t.Errorf("errors = %v, want the first two accepted and the third rejected with ErrBalanceTooLarge", errs)
	}
}

func TestValidateExpenseUpdateTakesOutTheStoredExpense(t *testing.T) {
	// Ana is at the limit, half a MaxAmount of it from the stored taxi.
	taxi := lent("ana", "beto", domain.MaxAmount/2)[0]
	taxi.ID = "taxi"
	ledger := tripWith(append(lent("ana", "beto", domain.MaxBalance-domain.MaxAmount/2), taxi), nil)
	edit := func(total int64) domain.Expense {
		e := lent("ana", "beto", total)[0]
		e.ID = "taxi"
		return e
	}

	if err := domain.ValidateExpenseUpdate(ledger, taxi, edit(domain.MaxAmount/2)); err != nil {
		t.Errorf("unchanged: ValidateExpenseUpdate = %v, want nil: the taxi counts once", err)
	}
	if err := domain.ValidateExpenseUpdate(ledger, taxi, edit(domain.MaxAmount/4)); err != nil {
		t.Errorf("lower: ValidateExpenseUpdate = %v, want nil", err)
	}
	if err := domain.ValidateExpenseUpdate(ledger, taxi, edit(domain.MaxAmount/2+1)); !errors.Is(err, domain.ErrBalanceTooLarge) {
		t.Errorf("one more cent: ValidateExpenseUpdate = %v, want ErrBalanceTooLarge", err)
	}
}

func TestRestoringARecordCountsItAgain(t *testing.T) {
	deletedAt := time.Date(2026, 10, 30, 12, 0, 0, 0, time.UTC)
	taxi := lent("ana", "beto", domain.MaxAmount)[0]
	taxi.ID, taxi.DeletedAt = "taxi", &deletedAt
	refund := domain.Settlement{ID: "refund", FromMemberID: "ana", ToMemberID: "beto", Amount: domain.MaxAmount, Currency: "USD", DeletedAt: &deletedAt}

	room := tripWith(append(lent("ana", "beto", domain.MaxBalance-domain.MaxAmount), taxi), []domain.Settlement{refund})
	if err := domain.ValidateExpense(room, taxi); err != nil {
		t.Errorf("restoring the taxi with room for it: ValidateExpense = %v, want nil", err)
	}
	if err := domain.ValidateSettlement(room, refund); err != nil {
		t.Errorf("restoring the refund with room for it: ValidateSettlement = %v, want nil", err)
	}

	full := tripWith(append(lent("ana", "beto", domain.MaxBalance-domain.MaxAmount+1), taxi), []domain.Settlement{refund})
	if err := domain.ValidateExpense(full, taxi); !errors.Is(err, domain.ErrBalanceTooLarge) {
		t.Errorf("restoring the taxi without room: ValidateExpense = %v, want ErrBalanceTooLarge", err)
	}
	if err := domain.ValidateSettlement(full, refund); !errors.Is(err, domain.ErrBalanceTooLarge) {
		t.Errorf("restoring the refund without room: ValidateSettlement = %v, want ErrBalanceTooLarge", err)
	}
}

func TestValidateSettlementKeepsBalancesWithinTheLimit(t *testing.T) {
	ledger := tripWith(lent("ana", "beto", domain.MaxBalance-1), nil)
	settlement := func(from, to string, amount int64) domain.Settlement {
		return domain.Settlement{ID: "s", FromMemberID: from, ToMemberID: to, Amount: amount, Currency: "USD"}
	}

	if err := domain.ValidateSettlement(ledger, settlement("beto", "ana", domain.MaxAmount)); err != nil {
		t.Errorf("Beto paying back: ValidateSettlement = %v, want nil", err)
	}
	if err := domain.ValidateSettlement(ledger, settlement("ana", "beto", 1)); err != nil {
		t.Errorf("to the limit: ValidateSettlement = %v, want nil", err)
	}
	if err := domain.ValidateSettlement(ledger, settlement("ana", "beto", 2)); !errors.Is(err, domain.ErrBalanceTooLarge) {
		t.Errorf("past the limit: ValidateSettlement = %v, want ErrBalanceTooLarge", err)
	}
}

func TestDeletingARecordKeepsBalancesWithinTheLimit(t *testing.T) {
	// The taxi took Ana up by MaxAmount; deleting it leaves what she owes Carla.
	taxi := lent("ana", "beto", domain.MaxAmount)[0]
	taxi.ID = "taxi"
	owing := func(amount int64) domain.Ledger { return tripWith(append(lent("carla", "ana", amount), taxi), nil) }

	if err := domain.ValidateExpenseDeletion(owing(domain.MaxBalance), taxi); err != nil {
		t.Errorf("Expense, to the limit: ValidateExpenseDeletion = %v, want nil", err)
	}
	if err := domain.ValidateExpenseDeletion(owing(domain.MaxBalance+1), taxi); !errors.Is(err, domain.ErrBalanceTooLarge) {
		t.Errorf("Expense, past the limit: ValidateExpenseDeletion = %v, want ErrBalanceTooLarge", err)
	}

	// The refund took Ana down by MaxAmount; deleting it leaves what Carla owes her.
	refund := domain.Settlement{ID: "refund", FromMemberID: "beto", ToMemberID: "ana", Amount: domain.MaxAmount, Currency: "USD"}
	owed := func(amount int64) domain.Ledger {
		return tripWith(lent("ana", "carla", amount), []domain.Settlement{refund})
	}

	if err := domain.ValidateSettlementDeletion(owed(domain.MaxBalance), refund); err != nil {
		t.Errorf("Settlement, to the limit: ValidateSettlementDeletion = %v, want nil", err)
	}
	if err := domain.ValidateSettlementDeletion(owed(domain.MaxBalance+1), refund); !errors.Is(err, domain.ErrBalanceTooLarge) {
		t.Errorf("Settlement, past the limit: ValidateSettlementDeletion = %v, want ErrBalanceTooLarge", err)
	}
}

func TestValidateChecksOnlyTheBalancesTheChangeMoves(t *testing.T) {
	// Carla's Balance is past the limit, as it could be in a ledger stored
	// before the limit existed. A record that leaves her out still goes in.
	ledger := tripWith(lent("carla", "dani", domain.MaxBalance+1), nil)

	if err := domain.ValidateExpense(ledger, dinner()); err != nil {
		t.Errorf("ValidateExpense = %v, want nil", err)
	}
}

func TestRelabelFailsWhenABalanceWouldGoBeyondTheLimit(t *testing.T) {
	// 10^12 yen are 10^15 fils: every amount fits, but ten of them owed by
	// Beto are 10^16 fils, past MaxBalance. Nine are not.
	yen := func(n int) []domain.Expense {
		var out []domain.Expense
		for i := range n {
			out = append(out, domain.Expense{
				ID: fmt.Sprint(i), Currency: "JPY", Total: 1_000_000_000_000, SplitMethod: domain.SplitExact,
				PaidShares: []domain.PaidShare{{MemberID: "ana", Amount: 1_000_000_000_000}},
				OwedShares: []domain.OwedShare{{MemberID: "beto", Amount: 1_000_000_000_000}},
			})
		}
		return out
	}

	if _, _, err := domain.Relabel(yen(9), nil, "KWD"); err != nil {
		t.Errorf("nine: Relabel = %v, want nil", err)
	}
	_, _, err := domain.Relabel(yen(10), nil, "KWD")
	var relabelErr *domain.RelabelError
	if !errors.Is(err, domain.ErrBalanceTooLarge) || errors.As(err, &relabelErr) {
		t.Errorf("ten: Relabel = %v, want ErrBalanceTooLarge", err)
	}
}
