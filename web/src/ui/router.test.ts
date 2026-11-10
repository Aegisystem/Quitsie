import { describe, expect, it } from 'vitest'
import { href, parseRoute, screenOf, type Route } from './router'

describe('routes', () => {
  it.each<Route>([
    { name: 'newExpense', ledgerId: 'trip', fromRejected: 'r1' },
    { name: 'newExpense', ledgerId: 'trip', part: 'split' },
    { name: 'editExpense', ledgerId: 'trip', expenseId: 'e1', part: 'paid' },
    { name: 'settle', ledgerId: 'trip', fromRejected: 'r2' },
    { name: 'settle', ledgerId: 'trip', from: 'ana', to: 'yo', amount: '4000', currency: 'USD' },
  ])('come back from their address: $name', (route) => {
    expect(parseRoute(href(route))).toEqual(route)
  })

  it('open a part of the Expense form over the same screen', () => {
    expect(href({ name: 'newExpense', ledgerId: 'trip', part: 'paid' })).toBe('#/expenses/new?ledger=trip&part=paid')
    expect(parseRoute('#/expenses/new?ledger=trip&part=other')).toEqual({ name: 'newExpense', ledgerId: 'trip' })
    expect(screenOf({ name: 'editExpense', ledgerId: 'trip', expenseId: 'e1', part: 'split' })).toBe(
      screenOf({ name: 'editExpense', ledgerId: 'trip', expenseId: 'e1' }),
    )
  })

  it('fix a rejected record with the form of its kind', () => {
    expect(href({ name: 'newExpense', ledgerId: 'trip', fromRejected: 'r1' })).toBe('#/expenses/new?ledger=trip&fix=r1')
    expect(href({ name: 'settle', ledgerId: 'trip', fromRejected: 'r2' })).toBe('#/settle?ledger=trip&fix=r2')
  })
})
