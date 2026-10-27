import { liveQuery, type Observable } from 'dexie'
import type { Activity, Debt, Expense, Member, Settlement } from '../api/client'
import type { Ledger, Replica } from './db'

export interface LedgerSummary {
  ledger: Ledger
  members: Member[]
}

// Home: every Group and Friendship the User can see.
export function liveHome(replica: Replica): Observable<LedgerSummary[]> {
  return liveQuery(async () => {
    const [ledgers, members] = await Promise.all([replica.ledgers.toArray(), replica.members.toArray()])
    return ledgers.map((ledger) => ({ ledger, members: members.filter((m) => m.ledgerId === ledger.id) }))
  })
}

// Synced: the server has it and a pull brought it into the replica.
export type ExpenseView = Expense & { sync: 'synced' }

// A ledger's Expenses, including deleted ones (with `deletedAt`).
export function liveExpenses(replica: Replica, ledgerId: string): Observable<ExpenseView[]> {
  return liveQuery(async () => {
    const expenses = await replica.expenses.where({ ledgerId }).toArray()
    return expenses.map((expense) => ({ ...expense, sync: 'synced' as const }))
  })
}

export type SettlementView = Settlement & { sync: 'synced' }

// A ledger's Settlements, including deleted ones (with `deletedAt`).
export function liveSettlements(replica: Replica, ledgerId: string): Observable<SettlementView[]> {
  return liveQuery(async () => {
    const settlements = await replica.settlements.where({ ledgerId }).toArray()
    return settlements.map((settlement) => ({ ...settlement, sync: 'synced' as const }))
  })
}

// The ledger's Debts as the server last computed them (Debt simplification only
// happens there, ADR 0003): empty until the first pull, or when all is settled.
export function liveDebts(replica: Replica, ledgerId: string): Observable<Debt[]> {
  return liveQuery(async () => (await replica.debts.get(ledgerId))?.debts ?? [])
}

// Who changed what, newest first: in every ledger, or only in `ledgerId`.
// Activities never change, so their version is the order they happened in.
export function liveActivity(replica: Replica, ledgerId?: string): Observable<Activity[]> {
  return liveQuery(async () => {
    const newestFirst = await replica.activities.orderBy('version').reverse().toArray()
    return ledgerId === undefined ? newestFirst : newestFirst.filter((activity) => activity.ledgerId === ledgerId)
  })
}
