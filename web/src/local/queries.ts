import { liveQuery, type Observable } from 'dexie'
import type { Activity, Debt, Expense, Member, Settlement } from '../api/client'
import type { Ledger, QueuedRecord, RejectedRecord, Replica, UnsyncedRecord } from './db'

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

// Synced: the server has it. Unsynced: it waits in the queue. Rejected: the
// server refused it, and it waits for the User to fix or discard it.
export type SyncState = 'synced' | 'unsynced' | 'rejected'

const syncState = ({ status }: QueuedRecord): SyncState => (status === 'accepted' ? 'synced' : status)

// An Expense of the replica, or one still in the queue, which has no
// `createdBy` or `version` yet and whose `createdAt` is when it was queued.
export type ExpenseView = Omit<Expense, 'createdBy' | 'version'> &
  Partial<Pick<Expense, 'createdBy' | 'version'>> & { sync: SyncState }

// A ledger's Expenses, the queued ones included, and deleted ones too (with `deletedAt`).
export function liveExpenses(replica: Replica, ledgerId: string): Observable<ExpenseView[]> {
  return liveQuery(async () => {
    const [synced, queued] = await Promise.all([
      replica.expenses.where({ ledgerId }).toArray(),
      replica.queue.where({ ledgerId }).toArray(),
    ])
    const views: ExpenseView[] = synced.map((expense) => ({ ...expense, sync: 'synced' }))
    const inReplica = new Set(synced.map(({ id }) => id))
    for (const record of queued) {
      if (record.kind !== 'expense' || inReplica.has(record.id)) continue
      views.push({ ...record.expense, createdAt: record.queuedAt, sync: syncState(record) })
    }
    return views
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

// The inbox of records created on the device that the server does not have
// yet, oldest first: the rejected ones wait for the User, the unsynced ones
// for the next sync.
export function liveRecords(replica: Replica): Observable<{ rejected: RejectedRecord[]; unsynced: UnsyncedRecord[] }> {
  return liveQuery(async () => {
    const queue = await replica.queue.toArray()
    return {
      rejected: queue.filter((record) => record.status === 'rejected'),
      unsynced: queue.filter((record) => record.status === 'unsynced'),
    }
  })
}
