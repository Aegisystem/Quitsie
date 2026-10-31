import { describe, expect, it } from 'vitest'
import type { Activity } from '../types'
import { describeActivity } from './activity'

const activity = (kind: string, details: Record<string, unknown>) =>
  ({ id: 'a', ledgerId: 'l', userId: 'u', kind, details, createdAt: '2026-10-31T10:00:00Z', version: 1 }) as Activity

const names = (memberId: string) => memberId.toUpperCase()

describe('describeActivity', () => {
  it('skips a kind this version does not know', () => {
    expect(describeActivity(activity('expense_archived', {}), names)).toBeUndefined()
  })

  it('tells turning Debt simplification on from turning it off', () => {
    expect(describeActivity(activity('debt_simplification_changed', { simplifyDebts: true }), names)?.sentence).toBe('debt_simplification_on')
    expect(describeActivity(activity('debt_simplification_changed', { simplifyDebts: false }), names)?.sentence).toBe('debt_simplification_off')
  })

  it('reads the description of an edited Expense from what it became', () => {
    const line = describeActivity(activity('expense_updated', { before: { description: 'Cena' }, after: { description: 'Cena del viernes' } }), names)
    expect(line?.values).toEqual({ description: 'Cena del viernes' })
  })

  it('names the Member an Activity is about', () => {
    expect(describeActivity(activity('member_removed', { memberId: 'beto' }), names)?.values).toEqual({ name: 'BETO' })
  })

  it('shows who gave what back to whom in a Settlement', () => {
    const details = { fromMemberId: 'ana', toMemberId: 'beto', amount: 2000, currency: 'JPY' }
    const values = describeActivity(activity('settlement_created', details), names)?.values
    expect(values).toMatchObject({ from: 'ANA', to: 'BETO' })
    expect(values?.amount).toMatch(/2.000/)
  })
})
