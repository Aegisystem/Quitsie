import type { Client } from 'openapi-fetch'
import type { NewExpense } from '../api/client'
import type { paths } from '../api/schema.gen'
import { type Replica, uuidv7 } from '../local'
import { pull } from './pull'
import { push } from './push'

export type Api = Client<paths>

export interface SyncEngine {
  // Sends the queue, then brings the replica up to date with the server.
  sync(): Promise<void>
  // Queues an Expense created on the device and returns the id it was given.
  addExpense(expense: Omit<NewExpense, 'id'>): Promise<string>
}

export function createSyncEngine({ replica, api }: { replica: Replica; api: Api }): SyncEngine {
  return {
    async sync() {
      await push(replica, api)
      await pull(replica, api)
    },

    async addExpense(expense) {
      const id = uuidv7()
      await replica.queue.add({
        id,
        ledgerId: expense.ledgerId,
        queuedAt: new Date().toISOString(),
        kind: 'expense',
        expense: { ...expense, id },
        status: 'unsynced',
      })
      return id
    },
  }
}
