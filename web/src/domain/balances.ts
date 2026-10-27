import type { CurrencyCode } from './currencies'
import type { ParticipantId } from './split'

interface Share {
  participantId: ParticipantId
  amount: number
}

// What the Balances need from an Expense, in the replica or in the queue.
export interface ExpenseRecord {
  id: string
  currency: CurrencyCode
  paidShares: readonly Share[]
  owedShares: readonly Share[]
  deletedAt?: string | null
}

// A Settlement records that `from` gave `amount` back to `to`.
export interface SettlementRecord {
  id: string
  from: ParticipantId
  to: ParticipantId
  amount: number
  currency: CurrencyCode
  deletedAt?: string | null
}

export interface Records {
  expenses: readonly ExpenseRecord[]
  settlements: readonly SettlementRecord[]
}

// Positive when the Participant is owed money in that currency, negative when
// they owe it. Balances in different currencies are never converted.
export interface Balance {
  participantId: ParticipantId
  currency: CurrencyCode
  amount: number
}

// Records still in the queue after the server accepted them and a pull brought
// them into the replica count once, as the replica has them.
function withoutSynced<T extends { id: string }>(replica: readonly T[], unsynced: readonly T[]): T[] {
  const synced = new Set(replica.map(({ id }) => id))
  return [...replica, ...unsynced.filter(({ id }) => !synced.has(id))]
}

// The Balances the device can show offline: plain sums over the replica and the
// UnsyncedRecords (ADR 0003). Debt simplification only happens on the server.
// Deleted Expenses and Settlements no longer count, and settled Balances are left out.
export function provisionalBalances(replica: Records, unsynced: Records): Balance[] {
  const totals = new Map<string, { participantId: ParticipantId; currency: CurrencyCode; amount: bigint }>()
  const add = (participantId: ParticipantId, currency: CurrencyCode, amount: number) => {
    const key = JSON.stringify([participantId, currency])
    const balance = totals.get(key) ?? { participantId, currency, amount: 0n }
    balance.amount += BigInt(amount)
    totals.set(key, balance)
  }

  for (const expense of withoutSynced(replica.expenses, unsynced.expenses)) {
    if (expense.deletedAt) continue
    for (const { participantId, amount } of expense.paidShares) add(participantId, expense.currency, amount)
    for (const { participantId, amount } of expense.owedShares) add(participantId, expense.currency, -amount)
  }
  for (const settlement of withoutSynced(replica.settlements, unsynced.settlements)) {
    if (settlement.deletedAt) continue
    add(settlement.from, settlement.currency, settlement.amount)
    add(settlement.to, settlement.currency, -settlement.amount)
  }

  return [...totals.values()]
    .filter(({ amount }) => amount !== 0n)
    .map(({ participantId, currency, amount }) => ({ participantId, currency, amount: Number(amount) }))
    .sort((a, b) =>
      a.participantId === b.participantId
        ? a.currency < b.currency ? -1 : 1
        : a.participantId < b.participantId ? -1 : 1,
    )
}
