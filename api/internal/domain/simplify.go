package domain

import (
	"cmp"
	"slices"
)

// SimplifyDebts gives the Debts of a ledger with debt simplification: a set of
// payments that settles every Balance, without changing any of them, in as few
// payments as this method finds. Each currency is settled on its own.
//
// It is greedy: the Member who owes the most pays the Member who is owed the
// most, as much as one of them can, until nobody owes anything. Each payment
// settles at least one Member, so n Members with a non-zero Balance need at
// most n-1 payments. That is an approximation: the true minimum is n minus the
// largest number of groups whose Balances add up to zero, which is NP-hard to
// find, and greedy can miss such groups. Ties go to the first Member by ID, so
// the same Balances always give the same Debts.
//
// The Balances of each currency must add up to zero, as Balances returns them.
// The Debts come sorted by currency, debtor and creditor.
func SimplifyDebts(balances []Balance) []Debt {
	byCurrency := make(map[Currency][]Balance)
	for _, b := range balances {
		byCurrency[b.Currency] = append(byCurrency[b.Currency], b)
	}

	var debts []Debt
	for currency, balances := range byCurrency {
		var debtors, creditors []position
		for _, b := range balances {
			switch {
			case b.Amount < 0:
				debtors = append(debtors, position{b.MemberID, -b.Amount})
			case b.Amount > 0:
				creditors = append(creditors, position{b.MemberID, b.Amount})
			}
		}

		for len(debtors) > 0 && len(creditors) > 0 {
			slices.SortFunc(debtors, largestFirst)
			slices.SortFunc(creditors, largestFirst)
			debtor, creditor := &debtors[0], &creditors[0]
			amount := min(debtor.amount, creditor.amount)
			debts = append(debts, Debt{FromMemberID: debtor.memberID, ToMemberID: creditor.memberID, Currency: currency, Amount: amount})
			debtor.amount -= amount
			creditor.amount -= amount
			debtors = slices.DeleteFunc(debtors, settled)
			creditors = slices.DeleteFunc(creditors, settled)
		}
	}
	sortDebts(debts)
	return debts
}

func largestFirst(a, b position) int {
	return cmp.Or(cmp.Compare(b.amount, a.amount), cmp.Compare(a.memberID, b.memberID))
}

func settled(p position) bool { return p.amount == 0 }
