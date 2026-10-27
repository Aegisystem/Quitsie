import { Dexie, type EntityTable } from 'dexie'

// The local replica and the queue of UnsyncedRecords live in IndexedDB, and the
// interface always reads from here, online or offline (docs/arquitectura.md).
// The tables for Groups, Expenses and so on arrive with the sync engine.

interface Meta {
  key: string // for example 'syncCursor'
  value: unknown
}

export const db = new Dexie('quitsie') as Dexie & {
  meta: EntityTable<Meta, 'key'>
}

db.version(1).stores({
  meta: 'key',
})
