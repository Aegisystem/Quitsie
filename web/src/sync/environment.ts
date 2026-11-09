import { diagnostics } from '../diagnostics'

// How long to wait before opening the event stream again after the server
// answered it with an error.
export const reopenEvents = 10_000

// What the engine needs from the browser. The tests pass their own, and decide
// by hand when the connection changes and when a timer fires.
export interface Environment {
  online(): boolean
  visible(): boolean
  // Calls `listener` on the event; the function it returns stops listening.
  listen(event: 'online' | 'visibilitychange', listener: () => void): () => void
  // Calls `callback` after `ms`; the function it returns cancels that.
  after(ms: number, callback: () => void): () => void
  // Opens the server's event stream (GET /api/v1/events, ADR 0013) and calls
  // `onChange` when a ledger of the User changed, and again each time the
  // stream reconnects, since what changed meanwhile is unknown. The function
  // it returns closes the stream.
  events?(onChange: () => void): () => void
}

export const browserEnvironment: Environment = {
  online: () => navigator.onLine,
  visible: () => document.visibilityState === 'visible',
  listen(event, listener) {
    const target = event === 'online' ? window : document
    target.addEventListener(event, listener)
    return () => target.removeEventListener(event, listener)
  },
  after(ms, callback) {
    const timer = setTimeout(callback, ms)
    return () => clearTimeout(timer)
  },
  events(onChange) {
    if (typeof EventSource === 'undefined') return () => {}
    let source: EventSource | undefined
    let reopen: ReturnType<typeof setTimeout> | undefined
    let opened = false
    let closed = false
    const open = () => {
      const current = new EventSource('/api/v1/events')
      source = current
      current.addEventListener('change', onChange)
      current.addEventListener('open', () => {
        // The first opening comes with the sync of start(); a reopening, after
        // the stream dropped, may have missed changes.
        if (opened) {
          diagnostics.report('events', 'volvió a conectar')
          onChange()
        }
        opened = true
      })
      current.addEventListener('error', () => {
        // The browser opens the stream again on its own after it drops, but
        // gives up for good when the server answers with an error, such as a
        // 502 while it restarts. Then the stream opens again here. Leaving
        // the page closes the stream the same way, so the log only says so
        // once the page is still there to reopen it.
        if (closed || current !== source || current.readyState !== EventSource.CLOSED) return
        current.close()
        reopen = setTimeout(() => {
          diagnostics.report('events', `se cerró por un error y se abrió de nuevo a los ${reopenEvents / 1000} s`)
          open()
        }, reopenEvents)
      })
    }
    open()
    return () => {
      closed = true
      clearTimeout(reopen)
      source?.close()
    }
  },
}
