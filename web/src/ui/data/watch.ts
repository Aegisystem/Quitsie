import { type DiagnosticSource, describeError, diagnostics } from '../../diagnostics'

// How long a query of the replica may take before the diagnostics log says so.
export const slowQuery = 5000
const firstRetry = 1000
const longestRetry = 30_000

export interface Clock {
  now(): number
  setTimeout(fire: () => void, ms: number): unknown
  clearTimeout(timer: unknown): void
}

const realClock: Clock = {
  now: () => Date.now(),
  setTimeout: (fire, ms) => setTimeout(fire, ms),
  clearTimeout: (timer) => clearTimeout(timer as ReturnType<typeof setTimeout>),
}

// What watchQuery needs of a live query: Dexie's Observable has it.
export interface Subscribable<T> {
  subscribe(observer: { next(value: T): void; error(error: unknown): void }): { unsubscribe(): void }
}

export interface WatchOptions {
  report?: (source: DiagnosticSource, message: string) => void
  clock?: Clock
}

// Follows a live query of the replica until the returned function stops it.
// A query that fails is tried again after 1 s, 2 s, 4 s and so on up to 30 s,
// instead of leaving its screen empty until a reload. Failures, and a first
// answer that takes more than slowQuery, go to the diagnostics log under
// `name`, which says what the query reads but carries no id.
export function watchQuery<T>(name: string, query: () => Subscribable<T>, onValue: (value: T) => void, options: WatchOptions = {}): () => void {
  const report = options.report ?? diagnostics.report
  const clock = options.clock ?? realClock
  const started = clock.now()
  let answered = false
  let failures = 0
  let stopped = false
  let unsubscribe = () => {}
  let retry: unknown

  const slow = clock.setTimeout(() => report('query', `${name}: sin respuesta en ${slowQuery / 1000} s`), slowQuery)

  function subscribe() {
    const subscription = query().subscribe({
      next(value) {
        if (!answered) {
          answered = true
          clock.clearTimeout(slow)
          const took = clock.now() - started
          if (took >= slowQuery) report('query', `${name}: respondió en ${Math.round(took / 1000)} s`)
        }
        failures = 0
        onValue(value)
      },
      error(error) {
        report('query', `${name}: ${describeError(error)}`)
        if (stopped) return
        retry = clock.setTimeout(subscribe, Math.min(firstRetry * 2 ** failures, longestRetry))
        failures += 1
      },
    })
    unsubscribe = () => subscription.unsubscribe()
  }

  subscribe()
  return () => {
    stopped = true
    unsubscribe()
    clock.clearTimeout(slow)
    clock.clearTimeout(retry)
  }
}
