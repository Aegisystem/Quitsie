import { describe, expect, it } from 'vitest'
import { type ExpenseRecord, provisionalBalances, type SettlementRecord } from './balances'

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
