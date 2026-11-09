import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { browserEnvironment, reopenEvents } from './environment'

// The parts of EventSource the environment uses; `fail` is an error event,
// after which the browser either retries (CONNECTING) or gives up (CLOSED).
class FakeEventSource extends EventTarget {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSED = 2
  static all: FakeEventSource[] = []
  readyState = FakeEventSource.CONNECTING
  readonly url: string
  constructor(url: string) {
    super()
    this.url = url
    FakeEventSource.all.push(this)
  }
  close() {
    this.readyState = FakeEventSource.CLOSED
  }
  opens() {
    this.readyState = FakeEventSource.OPEN
    this.dispatchEvent(new Event('open'))
  }
  fail(state: number) {
    this.readyState = state
    this.dispatchEvent(new Event('error'))
  }
  change() {
    this.dispatchEvent(new MessageEvent('change', { data: '{}' }))
  }
}

beforeEach(() => {
  FakeEventSource.all = []
  vi.stubGlobal('EventSource', FakeEventSource)
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

const events = browserEnvironment.events!

describe('the event stream', () => {
  it('passes on each change, and a reopening, which may have missed some', () => {
    const onChange = vi.fn()
    events(onChange)
    const [stream] = FakeEventSource.all
    stream!.opens()
    expect(onChange).not.toHaveBeenCalled()
    stream!.change()
    stream!.fail(FakeEventSource.CONNECTING)
    stream!.opens()
    expect(onChange).toHaveBeenCalledTimes(2)
    // The browser reconnected that one itself.
    expect(FakeEventSource.all).toHaveLength(1)
  })

  it('opens again when the server answered with an error, and syncs', () => {
    const onChange = vi.fn()
    events(onChange)
    FakeEventSource.all[0]!.opens()
    FakeEventSource.all[0]!.fail(FakeEventSource.CLOSED)

    vi.advanceTimersByTime(reopenEvents - 1)
    expect(FakeEventSource.all).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(FakeEventSource.all).toHaveLength(2)
    FakeEventSource.all[1]!.opens()
    expect(onChange).toHaveBeenCalledTimes(1)
  })

  it('stays closed once the engine closes it', () => {
    const close = events(vi.fn())
    FakeEventSource.all[0]!.fail(FakeEventSource.CLOSED)
    close()
    vi.advanceTimersByTime(reopenEvents * 3)
    expect(FakeEventSource.all).toHaveLength(1)
  })
})
