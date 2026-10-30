import type { NewExpense, NewSettlement } from '../api/client'
import { type Replica, uuidv7 } from '../local'
import { type Api, SyncFailure } from './api'
import { browserEnvironment, type Environment } from './environment'
import { pull } from './pull'
import { push } from './push'

const firstWait = 1000
const longestWait = 5 * 60 * 1000

// `retrying`: the last sync failed and another one is scheduled.
// `signedOut`: the server refused the session, so nothing retries until the
// next sync, which the sign-in should start.
export type SyncStatus = 'idle' | 'syncing' | 'offline' | 'retrying' | 'signedOut'

export interface AddOptions {
  // The id of the RejectedRecord this one fixes, which leaves the queue with it.
  replacing?: string
}

export interface SyncEngine {
  // Syncs now and from then on when the app comes back or reconnects.
  start(): void
  // Stops listening and cancels the pending retry.
  stop(): void
  // Sends the queue, then brings the replica up to date with the server. Only
  // one runs at a time: asked for during another, it runs once that one ends.
  // It never rejects: a failure shows in the status and schedules a retry.
  // Call it after any operation that goes straight to the API.
  sync(): Promise<void>
  // Queues an Expense created on the device and returns the id it was given.
  addExpense(expense: Omit<NewExpense, 'id'>, options?: AddOptions): Promise<string>
  // The same for a Settlement.
  addSettlement(settlement: Omit<NewSettlement, 'id'>, options?: AddOptions): Promise<string>
  // Removes a RejectedRecord, which only the User can decide. Any other record
  // stays: it is on its way to the server.
  discardRecord(id: string): Promise<void>
  // For useSyncExternalStore.
  getStatus(): SyncStatus
  subscribe(listener: () => void): () => void
}

export function createSyncEngine({
  replica,
  api,
  environment = browserEnvironment,
}: {
  replica: Replica
  api: Api
  environment?: Environment
}): SyncEngine {
  let status: SyncStatus = 'idle'
  const listeners = new Set<() => void>()
  let failures = 0
  let cancelRetry = () => {}
  let stopped = false
  let stopListening = () => {}
  let running: Promise<void> | undefined
  let following: Promise<void> | undefined

  function setStatus(next: SyncStatus) {
    status = next
    for (const listener of listeners) listener()
  }

  // The pull runs even if the push failed: a broken POST /records should not
  // keep the replica from getting news.
  async function attempt() {
    let pushFailure: unknown
    await push(replica, api).catch((failure: unknown) => {
      pushFailure = failure
    })
    await pull(replica, api)
    if (pushFailure) throw pushFailure
  }

  async function run() {
    cancelRetry()
    if (!environment.online()) {
      setStatus('offline')
      return
    }
    setStatus('syncing')
    try {
      await attempt()
      failures = 0
      setStatus('idle')
    } catch (failure) {
      if (failure instanceof SyncFailure && failure.status === 401) {
        failures = 0
        setStatus('signedOut')
        return
      }
      if (!stopped) cancelRetry = environment.after(Math.min(firstWait * 2 ** failures, longestWait), () => void sync())
      failures += 1
      setStatus('retrying')
    }
  }

  function sync(): Promise<void> {
    if (!running) {
      running = run().finally(() => {
        running = undefined
      })
      return running
    }
    following ??= running.then(() => {
      following = undefined
      return sync()
    })
    return following
  }

  const discardRejected = (id: string) =>
    replica.queue.where({ id }).filter(({ status }) => status === 'rejected').delete()

  async function enqueue(
    id: string,
    ledgerId: string,
    record: { kind: 'expense'; expense: NewExpense } | { kind: 'settlement'; settlement: NewSettlement },
    { replacing }: AddOptions = {},
  ): Promise<string> {
    await replica.transaction('rw', replica.queue, async () => {
      if (replacing) await discardRejected(replacing)
      await replica.queue.add({ id, ledgerId, queuedAt: new Date().toISOString(), status: 'unsynced', ...record })
    })
    void sync()
    return id
  }

  function stop() {
    stopped = true
    stopListening()
    cancelRetry()
  }

  return {
    start() {
      stop()
      stopped = false
      const stops = [
        environment.listen('online', () => void sync()),
        environment.listen('visibilitychange', () => {
          if (environment.visible()) void sync()
        }),
      ]
      stopListening = () => stops.forEach((stopOne) => stopOne())
      void sync()
    },

    stop,
    sync,

    addExpense(expense, options) {
      const id = uuidv7()
      return enqueue(id, expense.ledgerId, { kind: 'expense', expense: { ...expense, id } }, options)
    },

    addSettlement(settlement, options) {
      const id = uuidv7()
      return enqueue(id, settlement.ledgerId, { kind: 'settlement', settlement: { ...settlement, id } }, options)
    },

    async discardRecord(id) {
      await discardRejected(id)
    },

    getStatus: () => status,
    subscribe(listener) {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
  }
}
