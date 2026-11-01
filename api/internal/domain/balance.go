package domain

import (
	"maps"
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
//
// The validators keep every Balance of a stored ledger within MaxBalance, so
// each one fits in its int64.
func Balances(expenses []Expense, settlements []Settlement) []Balance {
	var ledger Ledger
	ledger.AddUp(expenses, settlements)

	var balances []Balance
	for _, k := range slices.SortedFunc(maps.Keys(ledger.balances), compareBalanceKeys) {
		if amount := ledger.balances[k].int64(); amount != 0 {
			balances = append(balances, Balance{MemberID: k.memberID, Currency: k.currency, Amount: amount})
		}
	}
	return balances
}
