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
    const source = new EventSource('/api/v1/events')
    let opened = false
    source.addEventListener('change', onChange)
    source.addEventListener('open', () => {
      // The first opening comes with the sync of start(); a reopening, after
      // the stream dropped, may have missed changes.
      if (opened) onChange()
      opened = true
    })
    return () => source.close()
  },
}
