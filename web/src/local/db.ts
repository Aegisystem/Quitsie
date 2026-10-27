import { Dexie, type DexieOptions, type EntityTable } from 'dexie'
import type { Activity, Expense, Friendship, Group, LedgerDebts, Member, Settlement } from '../api/client'

// The local replica lives in IndexedDB, and the interface always reads from
// here, online or offline (docs/arquitectura.md). Rows keep the shape GET /sync
// gives them, so a pull only has to put them.

// A Group or a Friendship: both are ledgers (ADR 0008).
export type Ledger = (Group & { kind: 'group' }) | (Friendship & { kind: 'friendship' })

interface Meta {
  key: 'cursor'
  value: number
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
  })
  return replica
}
