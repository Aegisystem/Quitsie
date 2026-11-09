// A log of what went wrong on this device: local queries that fail or take
// long, failed syncs, the event stream dropping, and errors nobody caught.
// Cuenta shows it, so a User can send a screenshot of a problem that only
// happens on their phone. It lives in localStorage, because reloading is what
// usually makes the problem go away. It holds no data of the User's accounts:
// no names, amounts or ids.

export type DiagnosticSource = 'query' | 'sync' | 'events' | 'error' | 'app'

export interface DiagnosticEntry {
  at: string
  source: DiagnosticSource
  message: string
}

export interface DiagnosticStorage {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
  removeItem(key: string): void
}

export interface Diagnostics {
  report(source: DiagnosticSource, message: string): void
  entries(): DiagnosticEntry[]
  clear(): void
  subscribe(listener: () => void): () => void
}

// The newest entries it keeps.
export const maxEntries = 50
const storageKey = 'quitsie.diagnostics'

export function createDiagnostics(storage: DiagnosticStorage | undefined, now: () => Date = () => new Date()): Diagnostics {
  const listeners = new Set<() => void>()
  let cached: DiagnosticEntry[] | undefined

  // What storage holds now: another page of Quitsie may have added to it.
  function stored(): DiagnosticEntry[] {
    try {
      const parsed: unknown = JSON.parse(storage?.getItem(storageKey) ?? '[]')
      return Array.isArray(parsed) ? parsed.filter(isEntry) : []
    } catch {
      return cached ?? []
    }
  }

  // The same list until something changes, as useSyncExternalStore needs.
  function read(): DiagnosticEntry[] {
    cached ??= stored()
    return cached
  }

  function write(next: DiagnosticEntry[]) {
    cached = next
    try {
      storage?.setItem(storageKey, JSON.stringify(next))
    } catch {
      // Private mode or a full storage: the log lasts until the page closes.
    }
    for (const listener of listeners) listener()
  }

  return {
    report(source, message) {
      write([...stored(), { at: now().toISOString(), source, message }].slice(-maxEntries))
    },
    entries: read,
    clear() {
      try {
        storage?.removeItem(storageKey)
      } catch {
        // Nothing to remove.
      }
      cached = []
      for (const listener of listeners) listener()
    },
    subscribe(listener) {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
  }
}

function isEntry(value: unknown): value is DiagnosticEntry {
  if (typeof value !== 'object' || value === null) return false
  const entry = value as Partial<DiagnosticEntry>
  return typeof entry.at === 'string' && typeof entry.source === 'string' && typeof entry.message === 'string'
}

// An error as one line: its name and message, never its data.
export function describeError(error: unknown): string {
  if (error instanceof Error) return `${error.name}: ${error.message}`
  return String(error)
}

// Records the errors nobody caught and what the browser does to the page in
// the background: Chrome on Android freezes it, resumes it, or brings it back
// from its back-forward cache. Also answers other Quitsie pages of this browser
// that count how many are open (countOpenPages).
export function followPageLife() {
  window.addEventListener('error', (event) => diagnostics.report('error', describeError(event.error ?? event.message)))
  window.addEventListener('unhandledrejection', (event) => diagnostics.report('error', `sin atender: ${describeError(event.reason)}`))
  document.addEventListener('freeze', () => diagnostics.report('app', 'el navegador congeló la página'))
  document.addEventListener('resume', () => diagnostics.report('app', 'el navegador reanudó la página'))
  window.addEventListener('pageshow', (event) => {
    if (event.persisted) diagnostics.report('app', 'la página volvió de la caché del navegador')
  })
  if (typeof BroadcastChannel === 'undefined') return
  pages = new BroadcastChannel('quitsie-pages')
  pages.addEventListener('message', (event) => {
    if (event.data === 'ping') pages?.postMessage('pong')
  })
}

let pages: BroadcastChannel | undefined

// How many pages of Quitsie this browser has open, this one included: the
// installed app and a tab share the replica. Undefined if it cannot tell. A
// channel never hears its own messages, so only the other pages answer.
export function countOpenPages(wait = 500): Promise<number | undefined> {
  const channel = pages
  if (!channel) return Promise.resolve(undefined)
  let others = 0
  const count = (event: MessageEvent) => {
    if (event.data === 'pong') others += 1
  }
  channel.addEventListener('message', count)
  channel.postMessage('ping')
  return new Promise((resolve) =>
    setTimeout(() => {
      channel.removeEventListener('message', count)
      resolve(others + 1)
    }, wait),
  )
}

function browserStorage(): DiagnosticStorage | undefined {
  try {
    return typeof localStorage === 'undefined' ? undefined : localStorage
  } catch {
    return undefined
  }
}

export const diagnostics = createDiagnostics(browserStorage())
