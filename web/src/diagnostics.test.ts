import { describe, expect, it } from 'vitest'
import { createDiagnostics, type DiagnosticStorage, describeError, maxEntries } from './diagnostics'

function memory(): DiagnosticStorage & { data: Map<string, string> } {
  const data = new Map<string, string>()
  return {
    data,
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => void data.set(key, value),
    removeItem: (key) => void data.delete(key),
  }
}

const at = (iso: string) => () => new Date(iso)

describe('the diagnostics log', () => {
  it('keeps what happened across a reload', () => {
    const storage = memory()
    createDiagnostics(storage, at('2026-11-09T15:00:00Z')).report('query', 'ledger: no answer in 5 s')
    expect(createDiagnostics(storage).entries()).toEqual([{ at: '2026-11-09T15:00:00.000Z', source: 'query', message: 'ledger: no answer in 5 s' }])
  })

  it('adds to what another page of Quitsie wrote meanwhile', () => {
    const storage = memory()
    const app = createDiagnostics(storage)
    const tab = createDiagnostics(storage)
    app.entries()
    tab.report('events', 'from the tab')
    app.report('query', 'from the app')
    expect(app.entries().map(({ message }) => message)).toEqual(['from the tab', 'from the app'])
  })

  it('keeps only the newest entries', () => {
    const log = createDiagnostics(memory())
    for (let i = 0; i < maxEntries + 5; i++) log.report('sync', `failure ${i}`)
    const entries = log.entries()
    expect(entries).toHaveLength(maxEntries)
    expect(entries[0]?.message).toBe('failure 5')
    expect(entries.at(-1)?.message).toBe(`failure ${maxEntries + 4}`)
  })

  it('tells who listens, and empties without leaving its key behind', () => {
    const storage = memory()
    const log = createDiagnostics(storage)
    let calls = 0
    log.subscribe(() => calls++)
    log.report('events', 'dropped')
    log.clear()
    expect(calls).toBe(2)
    expect(log.entries()).toEqual([])
    expect([...storage.data.keys()]).toEqual([])
  })

  it('works without storage, or with a broken one', () => {
    const broken: DiagnosticStorage = {
      getItem: () => 'not json',
      setItem: () => {
        throw new DOMException('full', 'QuotaExceededError')
      },
      removeItem: () => {
        throw new Error('blocked')
      },
    }
    for (const log of [createDiagnostics(undefined), createDiagnostics(broken)]) {
      log.report('error', 'boom')
      expect(log.entries()).toHaveLength(1)
      log.clear()
      expect(log.entries()).toEqual([])
    }
  })
})

it('describes an error by its name and message', () => {
  expect(describeError(new DOMException('Database has been closed', 'DatabaseClosedError'))).toBe('DatabaseClosedError: Database has been closed')
  expect(describeError('plain')).toBe('plain')
})
