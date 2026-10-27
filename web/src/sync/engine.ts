import type { Client } from 'openapi-fetch'
import type { paths } from '../api/schema.gen'
import type { Replica } from '../local'
import { pull } from './pull'

export type Api = Client<paths>

export interface SyncEngine {
  // Brings the replica up to date with the server.
  sync(): Promise<void>
}

export function createSyncEngine({ replica, api }: { replica: Replica; api: Api }): SyncEngine {
  return {
    async sync() {
      await pull(replica, api)
    },
  }
}
