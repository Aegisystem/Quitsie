import type { Client } from 'openapi-fetch'
import type { paths } from '../api/schema.gen'

export type Api = Client<paths>

// The server answered, but not with what was asked for.
export class SyncFailure extends Error {
  status: number

  constructor(status: number) {
    super(`The server answered ${status}`)
    this.status = status
  }
}
