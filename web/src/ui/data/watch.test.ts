import { describe, expect, it } from 'vitest'
import { type Clock, type Subscribable, slowQuery, watchQuery } from './watch'

// A clock whose timers fire only when the test moves it.
function fakeClock() {
  let now = 0
  let timers: { at: number; fire: () => void; id: number }[] = []
  let ids = 0
  const clock: Clock = {
    now: () => now,
    setTimeout(fire, ms) {
      const id = ++ids
      timers.push({ at: now + ms, fire, id })
      return id
    },
    clearTimeout(id) {
      timers = timers.filter((t) => t.id !== id)
    },
  }
  function advance(ms: number) {
    now += ms
    for (const timer of timers.filter((t) => t.at <= now)) {
      timers = timers.filter((t) => t !== timer)
      timer.fire()
    }
  }
  return { clock, advance, pending: () => timers.length }
}

// A query whose answers the test gives: `answer` or `fail` the latest subscriber.
function fakeQuery<T>() {
  const subscribers: { next: (v: T) => void; error: (e: unknown) => void; closed: boolean }[] = []
  const query = (): Subscribable<T> => ({
    subscribe(observer) {
      const subscriber = { next: (v: T) => observer.next(v), error: (e: unknown) => observer.error(e), closed: false }
      subscribers.push(subscriber)
      return {
        unsubscribe() {
          subscriber.closed = true
        },
      }
    },
  })
  return {
    query,
    subscriptions: () => subscribers.length,
    answer: (value: T) => subscribers.at(-1)?.next(value),
    fail: (error: unknown) => subscribers.at(-1)?.error(error),
    open: () => subscribers.filter((s) => !s.closed).length,
  }
}

function setup() {
  const time = fakeClock()
  const log: string[] = []
  const values: number[] = []
  const q = fakeQuery<number>()
  const stop = watchQuery('ledger', q.query, (v) => values.push(v), { clock: time.clock, report: (source, message) => log.push(`${source} ${message}`) })
  return { ...time, log, values, q, stop }
}

describe('watchQuery', () => {
  it('passes on every answer and logs nothing when the replica answers at once', () => {
    const { q, values, log } = setup()
    q.answer(1)
    q.answer(2)
    expect(values).toEqual([1, 2])
    expect(log).toEqual([])
  })

  it('tries a failed query again instead of leaving the screen empty', () => {
    const { q, values, log, advance } = setup()
    q.fail(new DOMException('Database has been closed', 'DatabaseClosedError'))
    expect(log).toEqual(['query ledger: DatabaseClosedError: Database has been closed'])
    advance(999)
    expect(q.subscriptions()).toBe(1)
    advance(1)
    expect(q.subscriptions()).toBe(2)
    q.fail(new Error('again'))
    advance(1999)
    expect(q.subscriptions()).toBe(2)
    advance(1)
    q.answer(7)
    expect(values).toEqual([7])
  })

  it('logs a query that takes long, and when it answers', () => {
    const { q, log, advance } = setup()
    advance(slowQuery)
    expect(log).toEqual(['query ledger: sin respuesta en 5 s'])
    advance(7000)
    q.answer(1)
    expect(log).toEqual(['query ledger: sin respuesta en 5 s', 'query ledger: respondió en 12 s'])
  })

  it('stops for good when the screen goes away', () => {
    const { q, stop, pending, advance } = setup()
    q.fail(new Error('boom'))
    stop()
    advance(60_000)
    expect(q.subscriptions()).toBe(1)
    expect(q.open()).toBe(0)
    expect(pending()).toBe(0)
  })
})
