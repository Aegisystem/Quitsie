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

// No Balance goes past maxBalance minor units, either way: 2^53 − 1, up to
// which a number holds every integer exactly, so every Balance reaches the
// screen exact. The server refuses any change that would take one past it
// (domain.MaxBalance in api/internal/domain), and the forms check it first
// (docs/reglas-de-negocio.md).
export const maxBalance = Number.MAX_SAFE_INTEGER

type Totals = Map<string, { participantId: ParticipantId; currency: CurrencyCode; amount: bigint }>

// Adds to `totals` what `records` move each Balance by, exactly, or takes it
// off with `sign` -1n. Deleted records do not count.
function addUp(totals: Totals, { expenses, settlements }: Records, sign: 1n | -1n = 1n): void {
  const add = (participantId: ParticipantId, currency: CurrencyCode, amount: number) => {
    const key = JSON.stringify([participantId, currency])
    const balance = totals.get(key) ?? { participantId, currency, amount: 0n }
    balance.amount += sign * BigInt(amount)
    totals.set(key, balance)
  }

  for (const expense of expenses) {
    if (expense.deletedAt) continue
    for (const { participantId, amount } of expense.paidShares) add(participantId, expense.currency, amount)
    for (const { participantId, amount } of expense.owedShares) add(participantId, expense.currency, -amount)
  }
  for (const settlement of settlements) {
    if (settlement.deletedAt) continue
    add(settlement.from, settlement.currency, settlement.amount)
    add(settlement.to, settlement.currency, -settlement.amount)
  }
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
  const totals: Totals = new Map()
  addUp(totals, {
    expenses: withoutSynced(replica.expenses, unsynced.expenses),
    settlements: withoutSynced(replica.settlements, unsynced.settlements),
  })

  return [...totals.values()]
    .filter(({ amount }) => amount !== 0n)
    .map(({ participantId, currency, amount }) => ({ participantId, currency, amount: Number(amount) }))
    .sort((a, b) =>
      a.participantId === b.participantId
        ? a.currency < b.currency ? -1 : 1
        : a.participantId < b.participantId ? -1 : 1,
    )
}

const noRecords: Records = { expenses: [], settlements: [] }

// The first Participant whose Balance would go past maxBalance, either way, if
// `added` started counting and `removed` stopped, as when an edit replaces an
// Expense; undefined if none. `balances` are the current ones, as
// provisionalBalances gives them. Like the server, it only checks the
// Balances the change moves.
export function overMaxBalance(balances: readonly Balance[], added: Records, removed: Records = noRecords): ParticipantId | undefined {
  const moves: Totals = new Map()
  addUp(moves, added)
  addUp(moves, removed, -1n)
  const limit = BigInt(maxBalance)
  for (const { participantId, currency, amount } of moves.values()) {
    if (amount === 0n) continue
    // A current Balance within the limit is a safe integer, so it converts
    // exactly; one past it, rounded, is still past it.
    const current = balances.find((b) => b.participantId === participantId && b.currency === currency)?.amount ?? 0
    const after = BigInt(current) + amount
    if (after > limit || after < -limit) return participantId
  }
  return undefined
}
