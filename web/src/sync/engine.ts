import type { NewExpense, NewSettlement } from '../api/client'
import { type Replica, uuidv7 } from '../local'
import { type Api, type Requests, SyncFailure } from './api'
import { browserEnvironment, type Environment } from './environment'
import { pull } from './pull'
import { push } from './push'

const firstWait = 1000
const longestWait = 5 * 60 * 1000
// How long one request of a sync may take. A phone that loses the network
// mid-request, often in the background, may never get an answer, and every
// later sync would wait for it.
const requestTimeout = 60 * 1000

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
  report = () => {},
}: {
  replica: Replica
  api: Api
  environment?: Environment
  // Tells the diagnostics log why a sync failed.
  report?: (message: string) => void
}): SyncEngine {
  let status: SyncStatus = 'idle'
  const listeners = new Set<() => void>()
  let failures = 0
  let cancelRetry = () => {}
  let stopped = false
  let stopListening = () => {}
  let closeEvents = () => {}
  let running: Promise<void> | undefined
  let following: Promise<void> | undefined
  // Aborts the sync in flight, if any.
  let giveUp = () => {}

  function setStatus(next: SyncStatus) {
    status = next
    for (const listener of listeners) listener()
  }

  // Each request of a sync gets its own deadline, and giving up the sync
  // aborts the request in flight.
  function requestsOf(sync: AbortSignal): Requests {
    return async (send) => {
      const request = new AbortController()
      const abort = () => request.abort()
      if (sync.aborted) abort()
      sync.addEventListener('abort', abort)
      const cancel = environment.after(requestTimeout, () => {
        report(`una petición pasó de ${requestTimeout / 1000} s sin respuesta y se abandonó`)
        abort()
      })
      try {
        return await send(request.signal)
      } finally {
        cancel()
        sync.removeEventListener('abort', abort)
      }
    }
  }

  // The pull runs even if the push failed: a broken POST /records should not
  // keep the replica from getting news.
  async function attempt(request: Requests) {
    let pushFailure: unknown
    await push(replica, api, request).catch((failure: unknown) => {
      pushFailure = failure
    })
    await pull(replica, api, request)
    if (pushFailure) throw pushFailure
  }

  async function run() {
    cancelRetry()
    if (!environment.online()) {
      setStatus('offline')
      return
    }
    setStatus('syncing')
    const current = new AbortController()
    giveUp = () => current.abort()
    try {
      await attempt(requestsOf(current.signal))
      failures = 0
      setStatus('idle')
    } catch (failure) {
      if (failure instanceof SyncFailure && failure.status === 401) {
        failures = 0
        setStatus('signedOut')
        return
      }
      // An abort is the engine's own doing: a deadline, reported when it
      // passed, or giving up the sync on purpose.
      if (!isAbort(failure)) report(failure instanceof SyncFailure ? `el servidor respondió ${failure.status}` : describe(failure))
      if (!stopped) cancelRetry = environment.after(Math.min(firstWait * 2 ** failures, longestWait), () => void sync())
      failures += 1
      setStatus('retrying')
    } finally {
      giveUp = () => {}
    }
  }

  function describe(failure: unknown): string {
    return failure instanceof Error ? `${failure.name}: ${failure.message}` : String(failure)
  }

  function isAbort(failure: unknown): boolean {
    return failure instanceof Error && failure.name === 'AbortError'
  }

  // Coming back to the screen or to the network, a request still in flight
  // most likely went through a connection that is gone: the sync starts over.
  function startOver() {
    if (running) report('al volver, un sync en curso se abandonó y empezó de nuevo')
    giveUp()
    void sync()
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

  // The event stream is open while the app is on screen; it tells the
  // engine to sync when someone else changes one of the User's ledgers.
  function openEvents() {
    closeEvents()
    closeEvents = environment.events?.(() => void sync()) ?? (() => {})
  }

  function stop() {
    stopped = true
    stopListening()
    closeEvents()
    closeEvents = () => {}
    cancelRetry()
    giveUp()
  }

  return {
    start() {
      stop()
      stopped = false
      const stops = [
        environment.listen('online', startOver),
        environment.listen('visibilitychange', () => {
          if (!environment.visible()) return closeEvents()
          openEvents()
          startOver()
        }),
      ]
      stopListening = () => stops.forEach((stopOne) => stopOne())
      if (environment.visible()) openEvents()
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
