import type { Client } from 'openapi-fetch'
import type { paths } from '../api/schema.gen'

export type Api = Client<paths>

// Sends one request of a sync with the signal that gives it up: when its
// deadline passes, or when the engine gives up the whole sync.
export type Requests = <T>(send: (signal: AbortSignal) => Promise<T>) => Promise<T>

// Requests without a deadline, for calls outside the engine.
export const untimed: Requests = (send) => send(new AbortController().signal)

// The server answered, but not with what was asked for.
export class SyncFailure extends Error {
  status: number

  constructor(status: number) {
    super(`The server answered ${status}`)
    this.status = status
  }
}
