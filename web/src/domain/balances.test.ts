import { describe, expect, it } from 'vitest'
import { type Balance, type ExpenseRecord, maxBalance, overMaxBalance, provisionalBalances, type SettlementRecord } from './balances'

const nothing = { expenses: [], settlements: [] }

// Ana paid 90.00 for the three of them.
const dinner: ExpenseRecord = {
  id: 'dinner',
  currency: 'USD',
  paidShares: [{ participantId: 'ana', amount: 9000 }],
  owedShares: [
    { participantId: 'ana', amount: 3000 },
    { participantId: 'beto', amount: 3000 },
    { participantId: 'carla', amount: 3000 },
  ],
}

// Beto paid Ana back his 30.00.
const betoPaysAna: SettlementRecord = { id: 'beto-pays-ana', from: 'beto', to: 'ana', amount: 3000, currency: 'USD' }

describe('provisionalBalances', () => {
  it('credits what each Participant paid and debits what they owe', () => {
    expect(provisionalBalances({ expenses: [dinner], settlements: [] }, nothing)).toEqual([
      { participantId: 'ana', currency: 'USD', amount: 6000 },
      { participantId: 'beto', currency: 'USD', amount: -3000 },
      { participantId: 'carla', currency: 'USD', amount: -3000 },
    ])
  })

  it('moves a Settlement from the payer to the receiver and leaves out settled Balances', () => {
    expect(provisionalBalances({ expenses: [dinner], settlements: [betoPaysAna] }, nothing)).toEqual([
      { participantId: 'ana', currency: 'USD', amount: 3000 },
      { participantId: 'carla', currency: 'USD', amount: -3000 },
    ])
  })

  it('keeps one Balance per currency and never converts between them', () => {
    const taxi: ExpenseRecord = {
      id: 'taxi',
      currency: 'COP',
      paidShares: [{ participantId: 'beto', amount: 3000000 }],
      owedShares: [
        { participantId: 'ana', amount: 1500000 },
        { participantId: 'beto', amount: 1500000 },
      ],
    }
    expect(provisionalBalances({ expenses: [dinner, taxi], settlements: [] }, nothing)).toEqual([
      { participantId: 'ana', currency: 'COP', amount: -1500000 },
      { participantId: 'ana', currency: 'USD', amount: 6000 },
      { participantId: 'beto', currency: 'COP', amount: 1500000 },
      { participantId: 'beto', currency: 'USD', amount: -3000 },
      { participantId: 'carla', currency: 'USD', amount: -3000 },
    ])
  })

  it('ignores deleted Expenses and Settlements', () => {
    const replica = {
      expenses: [{ ...dinner, deletedAt: '2026-10-26T12:00:00Z' }],
      settlements: [{ ...betoPaysAna, deletedAt: '2026-10-26T12:00:00Z' }],
    }
    expect(provisionalBalances(replica, nothing)).toEqual([])
  })

  it('adds the UnsyncedRecords to the replica', () => {
    expect(provisionalBalances({ expenses: [dinner], settlements: [] }, { expenses: [], settlements: [betoPaysAna] })).toEqual([
      { participantId: 'ana', currency: 'USD', amount: 3000 },
      { participantId: 'carla', currency: 'USD', amount: -3000 },
    ])
  })

  it('counts once a record that already reached the replica but is still in the queue', () => {
    const queued = { ...betoPaysAna, amount: 1000 }
    expect(
      provisionalBalances({ expenses: [dinner], settlements: [betoPaysAna] }, { expenses: [dinner], settlements: [queued] }),
    ).toEqual([
      { participantId: 'ana', currency: 'USD', amount: 3000 },
      { participantId: 'carla', currency: 'USD', amount: -3000 },
    ])
  })
})

describe('overMaxBalance', () => {
  const max = 10 ** 15
  // Ana paid `amount` for Beto.
  const lent = (amount: number, id = 'lent'): ExpenseRecord => ({
    id,
    currency: 'USD',
    paidShares: [{ participantId: 'ana', amount }],
    owedShares: [{ participantId: 'beto', amount }],
  })
  const at = (ana: number): Balance[] => [
    { participantId: 'ana', currency: 'USD', amount: ana },
    { participantId: 'beto', currency: 'USD', amount: -ana },
  ]
  const only = (record: ExpenseRecord | SettlementRecord) =>
    'paidShares' in record ? { expenses: [record], settlements: [] } : { expenses: [], settlements: [record] }

  it('is the same limit as the server: 2^53 − 1', () => {
    expect(maxBalance).toBe(9007199254740991)
  })

  it('lets a Balance reach the limit exactly, either way', () => {
    expect(overMaxBalance(at(maxBalance - max), only(lent(max)))).toBeUndefined()
  })

  it('names who would go one past the limit, in favor or against', () => {
    expect(overMaxBalance(at(maxBalance - max + 1), only(lent(max)))).toBe('ana')
    const betoOwes: Balance[] = [{ participantId: 'beto', currency: 'USD', amount: -(maxBalance - max + 1) }]
    expect(overMaxBalance(betoOwes, only(lent(max)))).toBe('beto')
  })

  it('checks a Settlement too', () => {
    const anaPaysBeto: SettlementRecord = { id: 's', from: 'ana', to: 'beto', amount: 2, currency: 'USD' }
    expect(overMaxBalance(at(maxBalance - 2), only(anaPaysBeto))).toBeUndefined()
    expect(overMaxBalance(at(maxBalance - 1), only(anaPaysBeto))).toBe('ana')
  })

  it('takes out the Expense an edit replaces, so that it does not count twice', () => {
    const stored = lent(max / 2, 'taxi')
    expect(overMaxBalance(at(maxBalance), only(lent(max / 2, 'taxi')), only(stored))).toBeUndefined()
    expect(overMaxBalance(at(maxBalance), only(lent(max / 2 + 1, 'taxi')), only(stored))).toBe('ana')
  })

  it('only checks the Balances the record moves, as the server does', () => {
    const carlaPastIt: Balance[] = [{ participantId: 'carla', currency: 'USD', amount: maxBalance + 2 }]
    expect(overMaxBalance(carlaPastIt, only(lent(max)))).toBeUndefined()
  })

  it('keeps each currency apart', () => {
    expect(overMaxBalance(at(maxBalance), only({ ...lent(max), currency: 'EUR' }))).toBeUndefined()
  })
})
