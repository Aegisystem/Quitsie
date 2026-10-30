import { liveQuery, type Observable } from 'dexie'
import type { Activity, Debt, Expense, Member, NewExpense, NewSettlement, Settlement } from '../api/client'
import { type Balance, type ExpenseRecord, provisionalBalances, type SettlementRecord } from '../domain'
import type { Ledger, QueuedRecord, RejectedRecord, Replica, UnsyncedRecord } from './db'

// Live queries: each one emits what the screens show and emits again whenever
// that changes, whether the change comes from a sync or from the queue. Reads
// never touch the network, so they work the same offline.

export interface LedgerSummary {
  ledger: Ledger
  members: Member[]
  balances: Balance[]
}

interface Stored {
  members: Member[]
  expenses: Expense[]
  settlements: Settlement[]
  queue: QueuedRecord[]
}

const expenseRecord = ({ id, currency, paidShares, owedShares, deletedAt }: NewExpense & { deletedAt?: string }): ExpenseRecord => ({
  id,
  currency,
  paidShares: paidShares.map(({ memberId, amount }) => ({ participantId: memberId, amount })),
  owedShares: owedShares.map(({ memberId, amount }) => ({ participantId: memberId, amount })),
  deletedAt,
})

const settlementRecord = ({ id, fromMemberId, toMemberId, amount, currency, deletedAt }: NewSettlement & { deletedAt?: string }): SettlementRecord => ({
  id,
  from: fromMemberId,
  to: toMemberId,
  amount,
  currency,
  deletedAt,
})

// Provisional Balances (ADR 0003): the replica plus the queue. Rejected records
// do not count until the User fixes them, and accepted ones count once.
function balances(ledgerId: string, { expenses, settlements, queue }: Stored): Balance[] {
  const counted = queue.filter((record) => record.ledgerId === ledgerId && record.status !== 'rejected')
  return provisionalBalances(
    {
      expenses: expenses.filter((expense) => expense.ledgerId === ledgerId).map(expenseRecord),
      settlements: settlements.filter((settlement) => settlement.ledgerId === ledgerId).map(settlementRecord),
    },
    {
      expenses: counted.flatMap((record) => (record.kind === 'expense' ? [expenseRecord(record.expense)] : [])),
      settlements: counted.flatMap((record) => (record.kind === 'settlement' ? [settlementRecord(record.settlement)] : [])),
    },
  )
}

function summary(ledger: Ledger, stored: Stored): LedgerSummary {
  return {
    ledger,
    members: stored.members.filter((member) => member.ledgerId === ledger.id),
    balances: balances(ledger.id, stored),
  }
}

async function readLedger(replica: Replica, ledgerId: string): Promise<Stored> {
  const [members, expenses, settlements, queue] = await Promise.all([
    replica.members.where({ ledgerId }).toArray(),
    replica.expenses.where({ ledgerId }).toArray(),
    replica.settlements.where({ ledgerId }).toArray(),
    replica.queue.where({ ledgerId }).toArray(),
  ])
  return { members, expenses, settlements, queue }
}

// Home: every Group and Friendship the User can see, in no particular order,
// each with its Members, former ones included, and the Balance of each Member.
// The screen finds the User's own Member by `userId`.
export function liveHome(replica: Replica): Observable<LedgerSummary[]> {
  return liveQuery(async () => {
    const [ledgers, members, expenses, settlements, queue] = await Promise.all([
      replica.ledgers.toArray(),
      replica.members.toArray(),
      replica.expenses.toArray(),
      replica.settlements.toArray(),
      replica.queue.toArray(),
    ])
    return ledgers.map((ledger) => summary(ledger, { members, expenses, settlements, queue }))
  })
}

// One Group or Friendship, or undefined if the User cannot see it (any more).
export function liveLedger(replica: Replica, ledgerId: string): Observable<LedgerSummary | undefined> {
  return liveQuery(async () => {
    const [ledger, stored] = await Promise.all([replica.ledgers.get(ledgerId), readLedger(replica, ledgerId)])
    return ledger && summary(ledger, stored)
  })
}

// The provisional Balances of a ledger, one per Member and currency, leaving out
// those at zero. Amounts are positive when the Member is owed money.
export function liveBalances(replica: Replica, ledgerId: string): Observable<Balance[]> {
  return liveQuery(async () => balances(ledgerId, await readLedger(replica, ledgerId)))
}

// Synced: the server has it. Unsynced: it waits in the queue. Rejected: the
// server refused it, and it waits for the User to fix or discard it.
export type SyncState = 'synced' | 'unsynced' | 'rejected'

const syncState = ({ status }: QueuedRecord): SyncState => (status === 'accepted' ? 'synced' : status)

// A row of the replica, or a record still in the queue, which has no `createdBy`
// or `version` yet and whose `createdAt` is when it was queued.
type View<Row> = Omit<Row, 'createdBy' | 'version'> & { createdBy?: string; version?: number; sync: SyncState }

export type ExpenseView = View<Expense>
export type SettlementView = View<Settlement>

// By the date the User gave them, then by when they were created.
function newestFirst<Row extends { date: string; createdAt: string }>(rows: Row[]): Row[] {
  return rows.sort((a, b) => b.date.localeCompare(a.date) || b.createdAt.localeCompare(a.createdAt))
}

// A ledger's Expenses, newest first: those in the replica, deleted ones included
// (with `deletedAt`), and those in the queue, rejected ones included.
export function liveExpenses(replica: Replica, ledgerId: string): Observable<ExpenseView[]> {
  return liveQuery(async () => {
    const { expenses, queue } = await readLedger(replica, ledgerId)
    const inReplica = new Set(expenses.map(({ id }) => id))
    const queued = queue.flatMap((record) =>
      record.kind === 'expense' && !inReplica.has(record.id)
        ? [{ ...record.expense, createdAt: record.queuedAt, sync: syncState(record) }]
        : [],
    )
    return newestFirst<ExpenseView>([...expenses.map((expense) => ({ ...expense, sync: 'synced' as const })), ...queued])
  })
}

// A ledger's Settlements, in the same way.
export function liveSettlements(replica: Replica, ledgerId: string): Observable<SettlementView[]> {
  return liveQuery(async () => {
    const { settlements, queue } = await readLedger(replica, ledgerId)
    const inReplica = new Set(settlements.map(({ id }) => id))
    const queued = queue.flatMap((record) =>
      record.kind === 'settlement' && !inReplica.has(record.id)
        ? [{ ...record.settlement, createdAt: record.queuedAt, sync: syncState(record) }]
        : [],
    )
    return newestFirst<SettlementView>([...settlements.map((settlement) => ({ ...settlement, sync: 'synced' as const })), ...queued])
  })
}

// The ledger's Debts as the server last computed them, so they do not include
// the queue: Debt simplification only happens there (ADR 0003). Empty until the
// first pull, or when everyone is settled.
export function liveDebts(replica: Replica, ledgerId: string): Observable<Debt[]> {
  return liveQuery(async () => (await replica.debts.get(ledgerId))?.debts ?? [])
}

// Who changed what, newest first: in every ledger, or only in `ledgerId`.
// Activities never change, so their version is the order they happened in.
export function liveActivity(replica: Replica, ledgerId?: string): Observable<Activity[]> {
  return liveQuery(async () => {
    const newest = await replica.activities.orderBy('version').reverse().toArray()
    return ledgerId === undefined ? newest : newest.filter((activity) => activity.ledgerId === ledgerId)
  })
}

// The inbox of records created on the device that the server does not have,
// oldest first: the rejected ones wait for the User, the unsynced ones for the
// next sync. Ids are UUIDv7, so the table's order is the order they were created.
export function liveRecords(replica: Replica): Observable<{ rejected: RejectedRecord[]; unsynced: UnsyncedRecord[] }> {
  return liveQuery(async () => {
    const queue = await replica.queue.toArray()
    return {
      rejected: queue.filter((record) => record.status === 'rejected'),
      unsynced: queue.filter((record) => record.status === 'unsynced'),
    }
  })
}
