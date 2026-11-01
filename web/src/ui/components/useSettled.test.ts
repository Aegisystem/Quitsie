import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useSettled } from './useSettled'

// React runs effects only with a DOM, and these tests run in Node
// (vitest.config.ts). This stands in for it with a single component: useState
// keeps its value between renders and setting it renders again; useEffect runs
// after a render in which its deps changed, once the last run is cleaned up.
const react = vi.hoisted(() => {
  let state: { value: unknown } | undefined
  let effect: { deps: unknown[]; cleanup: (() => void) | void } | undefined
  let pending: (() => void) | undefined
  let render = () => {}
  return {
    useState(initial: unknown) {
      const current = (state ??= { value: initial })
      const set = (value: unknown) => {
        current.value = value
        render()
      }
      return [current.value, set]
    },
    useEffect(run: () => (() => void) | void, deps: unknown[]) {
      if (effect?.deps.every((dep, i) => dep === deps[i])) return
      pending = () => {
        effect?.cleanup?.()
        effect = { deps, cleanup: run() }
      }
    },
    // Renders `component` now, and again on `rerender` or when its state
    // changes. `renders` has what each render returned, in order.
    mount<T>(component: () => T) {
      state = effect = pending = undefined
      const renders: T[] = []
      render = () => {
        renders.push(component())
        const run = pending
        pending = undefined
        run?.()
      }
      render()
      return { renders, rerender: () => render() }
    },
  }
})
vi.mock('react', () => ({ useState: react.useState, useEffect: react.useEffect }))

// A row of tokens whose tokens past the new end take 100 ms each to leave.
const key = (row: string[]) => row.join(' ')
const leavingMs = (shown: string[], now: string[]) => Math.max(0, shown.length - now.length) * 100

let reducedMotion = false

beforeEach(() => {
  vi.useFakeTimers()
  vi.stubGlobal('window', {
    setTimeout: (callback: () => void, ms: number) => setTimeout(callback, ms),
    clearTimeout: (timer: number) => clearTimeout(timer),
    matchMedia: () => ({ matches: reducedMotion }),
  })
})

afterEach(() => {
  reducedMotion = false
  vi.unstubAllGlobals()
  vi.useRealTimers()
})

function mountRow(first: string[]) {
  let now = first
  const view = react.mount(() => useSettled(now, key, leavingMs))
  return {
    renders: view.renders,
    shown: () => view.renders.at(-1),
    show(next: string[]) {
      now = next
      view.rerender()
    },
  }
}

describe('useSettled', () => {
  it('shows a new value at once when nothing leaves', () => {
    const row = mountRow(['a', 'b'])

    row.show(['a', 'b', 'c'])

    expect(row.shown()).toEqual(['a', 'b', 'c'])
  })

  it('keeps what was on screen until what leaves has had its time', () => {
    const row = mountRow(['a', 'b', 'c'])

    row.show(['a'])
    vi.advanceTimersByTime(199)
    expect(row.shown()).toEqual(['a', 'b', 'c'])

    vi.advanceTimersByTime(1)
    expect(row.shown()).toEqual(['a'])
  })

  it('starts the wait over from what is on screen when the value changes again', () => {
    const row = mountRow(['a', 'b', 'c'])

    row.show(['a'])
    vi.advanceTimersByTime(150)
    row.show(['a', 'b'])
    vi.advanceTimersByTime(99)
    expect(row.shown()).toEqual(['a', 'b', 'c'])

    vi.advanceTimersByTime(1)
    expect(row.shown()).toEqual(['a', 'b'])
  })

  it('does not start the wait over for an equal value', () => {
    const row = mountRow(['a', 'b', 'c'])

    row.show(['a'])
    vi.advanceTimersByTime(150)
    row.show(['a'])
    vi.advanceTimersByTime(50)

    expect(row.shown()).toEqual(['a'])
  })

  it('without motion, never shows what leaves', () => {
    reducedMotion = true
    const row = mountRow(['a', 'b', 'c'])
    const before = row.renders.length

    row.show(['a'])

    expect(row.renders.slice(before)).not.toContainEqual(['a', 'b', 'c'])
    expect(row.shown()).toEqual(['a'])
    expect(vi.getTimerCount()).toBe(0)
  })
})
