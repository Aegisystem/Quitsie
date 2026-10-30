// What the engine needs from the browser. The tests pass their own, and decide
// by hand when the connection changes and when a timer fires.
export interface Environment {
  online(): boolean
  visible(): boolean
  // Calls `listener` on the event; the function it returns stops listening.
  listen(event: 'online' | 'visibilitychange', listener: () => void): () => void
  // Calls `callback` after `ms`; the function it returns cancels that.
  after(ms: number, callback: () => void): () => void
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
}
