package domain

import (
	"cmp"
	"math/bits"
	"slices"
)

// Debt is an amount that FromMemberID owes ToMemberID, in minor units of Currency.
type Debt struct {
	FromMemberID string
	ToMemberID   string
	Currency     Currency
	Amount       int64
}

// PairwiseDebts gives the Debts of a ledger without debt simplification: who
// owes whom, Expense by Expense. Within an Expense, each Participant who owes
// more than they paid owes the Participants who paid more than they owe, in
// proportion to what each of those is owed. Debts between the same two Members
// offset each other, and Settlements reduce them. Deleted Expenses and
// Settlements do not count. The Debts come sorted by currency, debtor and creditor.
//
// It expects Expenses that ValidateExpense accepts. Any other never makes it
// panic, but then the Debts may not add up to the Balances.
func PairwiseDebts(expenses []Expense, settlements []Settlement) []Debt {
	ledger := make(pairwiseLedger)
	for _, s := range settlements {
		if s.DeletedAt == nil {
			// Paying back is like the payee now owing the payer.
			ledger.add(s.Currency, s.ToMemberID, s.FromMemberID, s.Amount)
		}
	}
	for _, e := range expenses {
		if e.DeletedAt != nil {
			continue
		}
		debtors, creditors := netPositions(e)
		for _, d := range debtors {
			for i, amount := range shareOut(d.amount, creditors) {
				if amount > 0 {
					ledger.add(e.Currency, d.memberID, creditors[i].memberID, amount)
				}
			}
		}
	}
	return ledger.debts()
}

// shareOut splits amount among creditors in proportion to what each one is
// still owed, and deducts it from them. Rounding down leaves a few minor units
// over; they go one each to the creditors with the largest remainders, and on
// a tie to the first Member by ID, so the same Expense always gives the same
// Debts. Since each debtor shares out in proportion to what is left, the last
// one pays exactly what remains and every creditor gets exactly what is owed.
func shareOut(amount int64, creditors []position) []int64 {
	owed := sumAmounts(creditors)
	// In a valid Expense a debtor never owes more than the creditors are still
	// owed; capping it keeps an invalid one from dividing by zero or overflowing.
	amount = min(amount, owed)
	shares := make([]int64, len(creditors))
	if amount == 0 {
		return shares
	}
	remainders := make([]int64, len(creditors))
	left := amount
	for i, c := range creditors {
		shares[i], remainders[i] = mulDiv(amount, c.amount, owed)
		left -= shares[i]
	}

	order := make([]int, len(creditors))
	for i := range order {
		order[i] = i
	}
	slices.SortStableFunc(order, func(i, j int) int {
		return cmp.Compare(remainders[j], remainders[i]) // largest remainder first; creditors are sorted by Member
	})
	for _, i := range order[:left] {
		shares[i]++
	}

	for i := range creditors {
		creditors[i].amount -= shares[i]
	}
	return shares
}

// mulDiv returns a*b/c and its remainder for non-negative a and b and positive
// c, with a*b computed in 128 bits so that it never overflows. It needs a <= c
// or b <= c, so that the quotient fits in an int64.
func mulDiv(a, b, c int64) (quotient, remainder int64) {
	hi, lo := bits.Mul64(uint64(a), uint64(b))
	q, r := bits.Div64(hi, lo, uint64(c))
	return int64(q), int64(r)
}

// position is how much a Member is owed (creditor) or owes (debtor) within one Expense.
type position struct {
	memberID string
	amount   int64
}

// netPositions nets what each Participant paid against what they owe and
// returns the debtors and the creditors, each sorted by Member.
func netPositions(e Expense) (debtors, creditors []position) {
	net := make(map[string]int64)
	for _, s := range e.PaidShares {
		net[s.MemberID] += s.Amount
	}
	for _, s := range e.OwedShares {
		net[s.MemberID] -= s.Amount
	}
	for id, amount := range net {
		switch {
		case amount < 0:
			debtors = append(debtors, position{id, -amount})
		case amount > 0:
			creditors = append(creditors, position{id, amount})
		}
	}
	byMember := func(a, b position) int { return cmp.Compare(a.memberID, b.memberID) }
	slices.SortFunc(debtors, byMember)
	slices.SortFunc(creditors, byMember)
	return debtors, creditors
}

func sumAmounts(positions []position) int64 {
	var sum int64
	for _, p := range positions {
		sum += p.amount
	}
	return sum
}

// pairwiseLedger keeps one signed amount per pair of Members and currency.
// For the pair (a, b) with a < b, a positive amount means a owes b.
type pairwiseLedger map[pairKey]int64

type pairKey struct {
	currency Currency
	a, b     string
}

// add records that from owes to amount.
func (l pairwiseLedger) add(currency Currency, from, to string, amount int64) {
	if from < to {
		l[pairKey{currency, from, to}] += amount
	} else {
		l[pairKey{currency, to, from}] -= amount
	}
}

func (l pairwiseLedger) debts() []Debt {
	var debts []Debt
	for k, amount := range l {
		switch {
		case amount > 0:
			debts = append(debts, Debt{FromMemberID: k.a, ToMemberID: k.b, Currency: k.currency, Amount: amount})
		case amount < 0:
			debts = append(debts, Debt{FromMemberID: k.b, ToMemberID: k.a, Currency: k.currency, Amount: -amount})
		}
	}
	sortDebts(debts)
	return debts
}

func sortDebts(debts []Debt) {
	slices.SortFunc(debts, func(x, y Debt) int {
		return cmp.Or(
			cmp.Compare(x.Currency, y.Currency),
			cmp.Compare(x.FromMemberID, y.FromMemberID),
			cmp.Compare(x.ToMemberID, y.ToMemberID),
		)
	})
}
