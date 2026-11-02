import { Dexie, type DexieOptions, type EntityTable, type Table } from 'dexie'
import type { Activity, Expense, Friendship, Group, LedgerDebts, Member, NewExpense, NewSettlement, RecordResult, Settlement } from '../api/client'

// The local replica lives in IndexedDB, and the interface always reads from
// here, online or offline (docs/arquitectura.md). Rows keep the shape GET /sync
// gives them, so a pull only has to put them.

// A Group or a Friendship: both are ledgers (ADR 0008).
export type Ledger = (Group & { kind: 'group' }) | (Friendship & { kind: 'friendship' })

// The queue of records created on the device (ADR 0001). An UnsyncedRecord
// waits to be sent; a RejectedRecord waits for the User to fix or discard it.
// An accepted record is out of the queue, and only waits for a pull to bring it
// into the replica, so it never vanishes from the screens in between.
export type QueuedRecord = { id: string; ledgerId: string; queuedAt: string } & (
  | { kind: 'expense'; expense: NewExpense }
  | { kind: 'settlement'; settlement: NewSettlement }
) &
  ({ status: 'unsynced' } | { status: 'rejected'; reason: RejectionReason; detail?: string } | { status: 'accepted' })

export type RejectionReason = NonNullable<RecordResult['reason']>
export type UnsyncedRecord = Extract<QueuedRecord, { status: 'unsynced' }>
export type RejectedRecord = Extract<QueuedRecord, { status: 'rejected' }>

interface Meta {
  key: 'cursor'
  value: number
}

// The queue of a User whose session ended by itself, set aside while someone
// else uses the device and hidden from them, until its owner signs in here
// again (docs/reglas-de-negocio.md, "Sin conexión").
export interface ParkedRecord {
  id: string
  owner: string
  record: QueuedRecord
}

// The User whose replica and queue these are.
interface Owner {
  key: 'replica'
  userId: string
}

export type Replica = Dexie & {
  ledgers: EntityTable<Ledger, 'id'>
  members: EntityTable<Member, 'id'>
  // Deleted Expenses and Settlements stay, with `deletedAt`, so they can be restored.
  expenses: EntityTable<Expense, 'id'>
  settlements: EntityTable<Settlement, 'id'>
  activities: EntityTable<Activity, 'id'>
  // The Debts the server computed for each ledger, already simplified when the
  // Group has Debt simplification on. They do not include UnsyncedRecords.
  debts: EntityTable<LedgerDebts, 'ledgerId'>
  meta: EntityTable<Meta, 'key'>
  // Table rather than EntityTable, whose insert type does not keep the union.
  queue: Table<QueuedRecord, string>
  parked: EntityTable<ParkedRecord, 'id'>
  owner: EntityTable<Owner, 'key'>
}

// `options` lets the tests pass an in-memory IndexedDB.
export function openReplica(name = 'quitsie', options?: DexieOptions): Replica {
  const replica = new Dexie(name, options) as Replica
  replica.version(2).stores({
    ledgers: 'id',
    members: 'id, ledgerId',
    expenses: 'id, ledgerId',
    settlements: 'id, ledgerId',
    activities: 'id, ledgerId, version',
    debts: 'ledgerId',
    meta: 'key',
    queue: 'id, ledgerId, status',
  })
  replica.version(3).stores({
    parked: 'id, owner',
    owner: 'key',
  })
  return replica
}

// Leaves nothing of the User on the device: the replica, the queue and the
// cursor, so the next sign-in starts from zero. Warning about the records that
// would be lost is up to the caller (docs/reglas-de-negocio.md). What other
// Users set aside stays: it is theirs.
export async function clearDevice(replica: Replica): Promise<void> {
  const tables = replica.tables.filter(({ name }) => name !== 'parked')
  await replica.transaction('rw', tables, async () => {
    await Promise.all(tables.map((table) => table.clear()))
  })
}

// Drops everything a pull can bring back, when the session ended by itself.
// The queue stays for its owner: it is the only thing the server lacks.
export async function releaseReplica(replica: Replica): Promise<void> {
  const tables = replica.tables.filter(({ name }) => !['queue', 'parked', 'owner'].includes(name))
  await replica.transaction('rw', tables, async () => {
    await Promise.all(tables.map((table) => table.clear()))
  })
}

// Whose replica and queue the device holds, if it knows.
export async function ownerOf(replica: Replica): Promise<string | undefined> {
  return (await replica.owner.get('replica'))?.userId
}

// Gives the device to userId in one transaction. The queue of whoever had it
// is set aside under their name, hidden; what userId set aside here before
// comes back to be sent; the rest goes, and the next pull rebuilds it.
// `previous` stands in for an owner the device did not record.
export async function handOver(replica: Replica, userId: string, previous?: string): Promise<void> {
  await replica.transaction('rw', replica.tables, async () => {
    const owner = (await ownerOf(replica)) ?? previous
    if (owner !== userId) {
      const left = await replica.queue.toArray()
      // Nobody can claim records of an unknown owner, but they are never dropped.
      await replica.parked.bulkPut(left.map((record) => ({ id: record.id, owner: owner ?? '', record })))
      await Promise.all(replica.tables.filter(({ name }) => name !== 'parked').map((table) => table.clear()))
      const back = replica.parked.where({ owner: userId })
      await replica.queue.bulkPut((await back.toArray()).map(({ record }) => record))
      await back.delete()
    }
    await replica.owner.put({ key: 'replica', userId })
  })
}
