import { describe, expect, it } from 'vitest'
import type { ExpenseView, Ledger, LedgerSummary, Member, SettlementView } from '../types'
import { balanceProof, friendOf, ledgerName, myBalance, myMember, myTotals, relabelAmount } from './ledgers'

const member = (id: string, userId?: string, status: Member['status'] = 'active'): Member => ({ id, ledgerId: 'l', name: id.toUpperCase(), userId, status, version: 1 })

const group: Ledger = { kind: 'group', id: 'g', name: 'Viaje', currency: 'EUR', simplifyDebts: true, createdAt: '', version: 1 }
const friendship: Ledger = { kind: 'friendship', id: 'f', createdAt: '', version: 1 }

const summary = (ledger: Ledger, members: Member[], balances: LedgerSummary['balances'] = []): LedgerSummary => ({ ledger, members, balances })

const expense = (over: Partial<ExpenseView>): ExpenseView => ({
  id: 'e',
  ledgerId: 'l',
  description: 'Cena',
  date: '2026-10-30',
  currency: 'EUR',
  total: 1000,
  splitMethod: 'equal',
  paidShares: [{ memberId: 'ana', amount: 1000 }],
  owedShares: [
    { memberId: 'ana', amount: 500 },
    { memberId: 'beto', amount: 500 },
  ],
  createdAt: '2026-10-30T10:00:00Z',
  sync: 'synced',
  ...over,
})

const settlement = (over: Partial<SettlementView>): SettlementView => ({
  id: 's',
  ledgerId: 'l',
  fromMemberId: 'beto',
  toMemberId: 'ana',
  amount: 200,
  currency: 'EUR',
  date: '2026-10-31',
  createdAt: '2026-10-31T10:00:00Z',
  sync: 'synced',
  ...over,
})

describe('myMember', () => {
  it('finds the active Member linked to the User', () => {
    expect(myMember([member('a', 'other'), member('b', 'me')], 'me')?.id).toBe('b')
  })

  it('does not count a former Member as the User', () => {
    expect(myMember([member('b', 'me', 'former')], 'me')).toBeUndefined()
  })
})

describe('ledgerName', () => {
  it('is the name of a Group', () => {
    expect(ledgerName(summary(group, []), 'me')).toBe('Viaje')
  })

  it('is the name of the Friend in a Friendship, pending or linked', () => {
    expect(ledgerName(summary(friendship, [member('a', 'me'), member('b')]), 'me')).toBe('B')
    expect(friendOf([member('a', 'other'), member('b', 'me')], 'me')?.id).toBe('a')
  })
})

describe('myBalance', () => {
  it('keeps only the Member and puts the preferred currency first', () => {
    const balances = [
      { participantId: 'a', currency: 'CLP', amount: 500 },
      { participantId: 'a', currency: 'USD', amount: -200 },
      { participantId: 'a', currency: 'EUR', amount: 100 },
      { participantId: 'b', currency: 'EUR', amount: -100 },
    ]
    expect([...myBalance({ balances }, 'a', 'USD')]).toEqual([
      ['USD', -200],
      ['CLP', 500],
      ['EUR', 100],
    ])
  })
})

describe('myTotals', () => {
  const user = { id: 'me', preferredCurrency: 'EUR' }

  it('adds up each currency across ledgers and never converts', () => {
    const home = [
      summary(group, [member('a', 'me')], [{ participantId: 'a', currency: 'EUR', amount: 100 }]),
      summary(friendship, [member('b', 'me')], [
        { participantId: 'b', currency: 'EUR', amount: 50 },
        { participantId: 'b', currency: 'USD', amount: -20 },
      ]),
    ]
    expect([...myTotals(home, user)]).toEqual([
      ['EUR', 150],
      ['USD', -20],
    ])
  })

  it('leaves out a currency that cancels out across ledgers', () => {
    const home = [
      summary(group, [member('a', 'me')], [{ participantId: 'a', currency: 'EUR', amount: 100 }]),
      summary(friendship, [member('b', 'me')], [{ participantId: 'b', currency: 'EUR', amount: -100 }]),
    ]
    expect(myTotals(home, user).size).toBe(0)
  })

  it('leaves out ledgers where the User is no longer a Member', () => {
    const home = [summary(group, [member('a', 'me', 'former')], [{ participantId: 'a', currency: 'EUR', amount: 100 }])]
    expect(myTotals(home, user).size).toBe(0)
  })
})

describe('balanceProof', () => {
  const names = (id: string) => id.toUpperCase()

  it('lists what moves the Balance, newest first, with its effect', () => {
    const lines = balanceProof('beto', [expense({})], [settlement({})], names)
    expect(lines.map(({ description, amount }) => [description, amount])).toEqual([
      ['BETO → ANA', 200],
      ['Cena', -500],
    ])
  })

  it('takes a Settlement off the Balance of whoever received it', () => {
    expect(balanceProof('ana', [], [settlement({})], names)[0]?.amount).toBe(-200)
  })

  it('leaves out deleted and rejected records, which do not count', () => {
    const expenses = [expense({ deletedAt: '2026-10-31T00:00:00Z' }), expense({ sync: 'rejected' })]
    const settlements = [settlement({ deletedAt: '2026-10-31T00:00:00Z' }), settlement({ sync: 'rejected' })]
    expect(balanceProof('ana', expenses, settlements, names)).toEqual([])
  })

  it('counts a record still in the queue', () => {
    expect(balanceProof('ana', [expense({ sync: 'unsynced' })], [], names)).toHaveLength(1)
  })

  it('leaves out records the Member is not in', () => {
    expect(balanceProof('carla', [expense({})], [settlement({})], names)).toEqual([])
  })
})

describe('relabelAmount', () => {
  it('keeps the number between currencies with the same decimals', () => {
    expect(relabelAmount(1050, 'USD', 'EUR')).toBe(1050)
  })

  it('keeps the number when the new currency has fewer decimals', () => {
    expect(relabelAmount(1000, 'USD', 'CLP')).toBe(10)
    expect(relabelAmount(1100, 'USD', 'CLP')).toBe(11)
  })

  it('refuses to lose minor units', () => {
    expect(relabelAmount(1050, 'USD', 'CLP')).toBeUndefined()
  })

  it('keeps the number when the new currency has more decimals', () => {
    expect(relabelAmount(10, 'CLP', 'USD')).toBe(1000)
  })
})
