import { Dexie, type DexieOptions, type EntityTable } from 'dexie'
import type { Friendship, Group, Member } from '../api/client'

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
  meta: EntityTable<Meta, 'key'>
}

// `options` lets the tests pass an in-memory IndexedDB.
export function openReplica(name = 'quitsie', options?: DexieOptions): Replica {
  const replica = new Dexie(name, options) as Replica
  replica.version(2).stores({
    ledgers: 'id',
    members: 'id, ledgerId',
    meta: 'key',
  })
  return replica
}
