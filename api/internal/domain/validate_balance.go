package domain

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
)

// MaxBalance is the largest a Balance can be, in either direction, in minor
// units of its currency: 2^53 - 1, a little over nine Expenses of MaxAmount.
// The client keeps amounts as JavaScript numbers, which hold integers exactly
// only up to there: past it, JSON.parse already rounds them
// (docs/reglas-de-negocio.md). web/src/domain/balances.ts has the same limit.
//
// The validators reject any change that would take a Balance it moves beyond
// it: creating, editing, deleting or restoring a record, and relabeling. As
// the Balances stay within it, so do the simplified Debts.
const MaxBalance = 1<<53 - 1

// AddUp counts expenses and settlements in the Balances that the validators
// check changes against. Deleted ones do not count, as in Balances. Call it
// with every record of the ledger before validating a change, and again with
// each record stored after that, so the next one is checked against them too.
func (l *Ledger) AddUp(expenses []Expense, settlements []Settlement) {
	if l.balances == nil {
		l.balances = make(balanceSums)
	}
	for _, e := range expenses {
		if e.DeletedAt == nil {
			l.balances.expense(e, false)
		}
	}
	for _, s := range settlements {
		if s.DeletedAt == nil {
			l.balances.settlement(s, false)
		}
	}
}

// checkBalances checks that a change keeps within MaxBalance every Balance it
// moves; moves holds how much it moves each one. A Balance the change does not
// move is not checked again.
func (l Ledger) checkBalances(moves balanceSums) error {
	for _, k := range slices.SortedFunc(maps.Keys(moves), compareBalanceKeys) {
		move := moves[k]
		if move == (sum{}) {
			continue
		}
		balance := l.balances[k]
		balance.addSum(move)
		if !balance.within(MaxBalance) {
			return fmt.Errorf("member %q in %s: %w", k.memberID, k.currency, ErrBalanceTooLarge)
		}
	}
	return nil
}

// balanceKey names one Balance: a Member's, in one currency.
type balanceKey struct {
	currency Currency
	memberID string
}

func compareBalanceKeys(a, b balanceKey) int {
	return cmp.Or(cmp.Compare(a.currency, b.currency), cmp.Compare(a.memberID, b.memberID))
}

// balanceSums are Balances added up exactly.
type balanceSums map[balanceKey]sum

// expense moves the Balances as e does: up by what each Participant paid and
// down by what each one owes. With undo it moves them back, as when e stops
// counting.
func (b balanceSums) expense(e Expense, undo bool) {
	for _, s := range e.PaidShares {
		b.move(e.Currency, s.MemberID, s.Amount, undo)
	}
	for _, s := range e.OwedShares {
		b.move(e.Currency, s.MemberID, s.Amount, !undo)
	}
}

// settlement moves the Balances as s does: paying back reduces what the payer
// owes and what the payee is owed. With undo it moves them back.
func (b balanceSums) settlement(s Settlement, undo bool) {
	b.move(s.Currency, s.FromMemberID, s.Amount, undo)
	b.move(s.Currency, s.ToMemberID, s.Amount, !undo)
}

// move moves the Balance of memberID in currency up by amount, or down.
func (b balanceSums) move(currency Currency, memberID string, amount int64, down bool) {
	k := balanceKey{currency, memberID}
	balance := b[k]
	if down {
		balance.sub(amount)
	} else {
		balance.add(amount)
	}
	b[k] = balance
}
