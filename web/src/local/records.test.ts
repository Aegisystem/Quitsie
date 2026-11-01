import { expect, test } from 'vitest'
import { provisionalBalances } from '../domain'
import { expenseRecord, settlementRecord } from './records'

test('an Expense counts for the Members who paid and owe, by their ids', () => {
  const dinner = expenseRecord({
    id: 'dinner',
    currency: 'USD',
    paidShares: [{ memberId: 'ana', amount: 9000 }],
    owedShares: [
      { memberId: 'ana', amount: 4500, splitValue: 1 },
      { memberId: 'beto', amount: 4500, splitValue: 1 },
    ],
  })

  expect(dinner).toEqual({
    id: 'dinner',
    currency: 'USD',
    paidShares: [{ participantId: 'ana', amount: 9000 }],
    owedShares: [
      { participantId: 'ana', amount: 4500 },
      { participantId: 'beto', amount: 4500 },
    ],
  })
})

test('a Settlement goes from one Member to the other', () => {
  expect(settlementRecord({ id: 'cash', fromMemberId: 'beto', toMemberId: 'ana', amount: 4500, currency: 'USD' })).toEqual({
    id: 'cash',
    from: 'beto',
    to: 'ana',
    amount: 4500,
    currency: 'USD',
  })
})

test('a record not saved yet has no id, and a deleted one stops counting', () => {
  const unsaved = settlementRecord({ fromMemberId: 'beto', toMemberId: 'ana', amount: 100, currency: 'USD' })
  const deleted = settlementRecord({ id: 'old', fromMemberId: 'ana', toMemberId: 'beto', amount: 100, currency: 'USD', deletedAt: '2026-10-27T10:00:00Z' })

  expect(unsaved.id).toBe('')
  expect(provisionalBalances({ expenses: [], settlements: [deleted] }, { expenses: [], settlements: [] })).toEqual([])
})
