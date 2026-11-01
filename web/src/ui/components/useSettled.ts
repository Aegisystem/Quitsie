import { useEffect, useState } from 'react'

// Without motion, what leaves goes at once, as everything else does.
function reducedMotion(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

// What to show of `now` while part of what was on screen leaves: the value last
// left on screen until `leavingMs(shown, now)` has passed, then `now`. With
// nothing leaving, or without motion, it is `now` at once. `key` stands for a
// value, so an equal one does not start the wait over.
export function useSettled<T>(now: T, key: (value: T) => string, leavingMs: (shown: T, now: T) => number): T {
  const [shown, setShown] = useState(now)
  const still = reducedMotion()
  const ms = still ? 0 : leavingMs(shown, now)
  const nowKey = key(now)
  const shownKey = key(shown)
  useEffect(() => {
    if (nowKey === shownKey) return
    const settle = () => setShown(now)
    if (ms === 0) {
      settle()
      return
    }
    const timer = window.setTimeout(settle, ms)
    return () => window.clearTimeout(timer)
    // The keys stand for the values themselves.
  }, [nowKey, shownKey])
  return still ? now : shown
}
