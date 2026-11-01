import { describe, expect, it } from 'vitest'
import { href, parseRoute, type Route } from './router'

describe('routes', () => {
  it.each<Route>([
    { name: 'newExpense', ledgerId: 'trip', fromRejected: 'r1' },
    { name: 'settle', ledgerId: 'trip', fromRejected: 'r2' },
    { name: 'settle', ledgerId: 'trip', from: 'ana', to: 'yo', amount: '4000', currency: 'USD' },
  ])('come back from their address: $name', (route) => {
    expect(parseRoute(href(route))).toEqual(route)
  })

  it('fix a rejected record with the form of its kind', () => {
    expect(href({ name: 'newExpense', ledgerId: 'trip', fromRejected: 'r1' })).toBe('#/expenses/new?ledger=trip&fix=r1')
    expect(href({ name: 'settle', ledgerId: 'trip', fromRejected: 'r2' })).toBe('#/settle?ledger=trip&fix=r2')
  })
})
