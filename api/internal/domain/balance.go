package domain

import (
	"cmp"
	"slices"
)

// Balance is what a Member has in their favor (positive) or against them
// (negative) in one currency. Balances are never converted between currencies.
type Balance struct {
	MemberID string
	Currency Currency
	Amount   int64
}

// Balances adds up the Expenses and Settlements of a ledger, except deleted
// ones, into one Balance per Member and currency. It leaves out zero Balances
// and sorts the rest by currency and then by Member.
func Balances(expenses []Expense, settlements []Settlement) []Balance {
	type key struct {
		currency Currency
		memberID string
	}
	amounts := make(map[key]int64)
	for _, e := range expenses {
		if e.DeletedAt != nil {
			continue
		}
		for _, s := range e.PaidShares {
			amounts[key{e.Currency, s.MemberID}] += s.Amount
		}
		for _, s := range e.OwedShares {
			amounts[key{e.Currency, s.MemberID}] -= s.Amount
		}
	}
	for _, s := range settlements {
		if s.DeletedAt != nil {
			continue
		}
		// Paying back reduces what the payer owes and what the payee is owed.
		amounts[key{s.Currency, s.FromMemberID}] += s.Amount
		amounts[key{s.Currency, s.ToMemberID}] -= s.Amount
	}

	var balances []Balance
	for k, amount := range amounts {
		if amount != 0 {
			balances = append(balances, Balance{MemberID: k.memberID, Currency: k.currency, Amount: amount})
		}
	}
	slices.SortFunc(balances, func(a, b Balance) int {
		return cmp.Or(cmp.Compare(a.Currency, b.Currency), cmp.Compare(a.MemberID, b.MemberID))
	})
	return balances
}
