import 'fake-indexeddb/auto'
import { describe, expect, it } from 'vitest'
import type { NewExpense } from '../api/client'
import { openReplica, type QueuedRecord, type Replica } from '../local'
import type { SyncEngine, SyncStatus } from '../sync'
import { createDeviceData } from './replica'
import type { KeyValueStorage } from './session'

let databases = 0

// Only what the device data uses of the engine, with its status in the test's hands.
function fakeEngine(onSync: () => void = () => {}) {
  let status: SyncStatus = 'idle'
  const listeners = new Set<() => void>()
  const engine = {
    stops: 0,
    stop() {
      this.stops++
    },
    getStatus: () => status,
    subscribe(listener: () => void) {
      listeners.add(listener)
      return () => listeners.delete(listener)
    },
    sync: async () => onSync(),
  }
  return {
    engine: engine as typeof engine & SyncEngine,
    setStatus(next: SyncStatus) {
      status = next
      for (const listener of listeners) listener()
    },
  }
}

function memoryStorage(): KeyValueStorage & { data: Map<string, string> } {
  const data = new Map<string, string>()
  return {
    data,
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => void data.set(key, value),
    removeItem: (key) => void data.delete(key),
  }
}

const queued = (id: string, status: 'unsynced' | 'accepted' | 'rejected'): QueuedRecord => ({
  id,
  ledgerId: 'ledger',
  queuedAt: '2026-10-31T12:00:00Z',
  kind: 'expense',
  expense: {} as NewExpense,
  ...(status === 'rejected' ? { status, reason: 'currency_mismatch' as const } : { status }),
})

async function setup(onSync?: () => void) {
  const replica: Replica = openReplica(`device-test-${++databases}`)
  const { engine, setStatus } = fakeEngine(onSync)
  const storage = memoryStorage()
  await replica.ledgers.put({ id: 'ledger', kind: 'friendship', createdAt: '2026-10-31T12:00:00Z', version: 1 })
  await replica.meta.put({ key: 'cursor', value: 7 })
  await replica.queue.put(queued('record', 'unsynced'))
  return { replica, engine, setStatus, storage, device: createDeviceData({ replica, engine, storage }) }
}

async function isEmpty(replica: Replica) {
  const counts = await Promise.all(replica.tables.map((table) => table.count()))
  return counts.every((count) => count === 0)
}

describe('device data', () => {
  it('signing out leaves nothing: replica, queue and cursor', async () => {
    const { replica, device, engine } = await setup()
    await device.clear()
    expect(await isEmpty(replica)).toBe(true)
    expect(engine.stops).toBe(1)
  })

  it('waits for a sync on its way, whose pull would write after the wipe', async () => {
    const { replica, device, setStatus } = await setup()
    setStatus('syncing')
    const clearing = device.clear()
    await replica.ledgers.put({ id: 'pulled', kind: 'friendship', createdAt: '2026-10-31T12:00:00Z', version: 2 })
    setStatus('idle')
    await clearing
    expect(await replica.ledgers.count()).toBe(0)
  })

  it('keeps everything for the User who owns it', async () => {
    const { replica, device } = await setup()
    await device.admit('ana')
    await replica.queue.put(queued('mine', 'unsynced'))
    await device.admit('ana')
    expect(await replica.queue.get('mine')).toBeDefined()
  })

  it('never shows another User what the last one left on the device', async () => {
    const { replica, device, storage } = await setup()
    await device.admit('ana')
    await replica.queue.put(queued('anas', 'unsynced'))
    await device.admit('beto')
    expect(await isEmpty(replica)).toBe(true)
    expect(storage.data.get('quitsie.replica.owner')).toBe('beto')
  })

  it('tries the wipe again at the next sign-in if it failed', async () => {
    const { replica, device, storage } = await setup()
    await device.admit('ana')
    const clear = replica.transaction.bind(replica)
    replica.transaction = (() => Promise.reject(new Error('blocked'))) as unknown as Replica['transaction']
    await device.admit('beto')
    expect(storage.data.get('quitsie.replica.owner')).toBe('ana')
    replica.transaction = clear
    await device.admit('beto')
    expect(await isEmpty(replica)).toBe(true)
  })

  it('counts what could not be sent, after trying to send it', async () => {
    const pending = await setup()
    await pending.replica.queue.bulkPut([queued('rejected', 'rejected'), queued('accepted', 'accepted')])
    expect(await pending.device.records()).toEqual({ unsynced: 1, rejected: 1 })

    const sent = await setup(() => void sent.replica.queue.clear())
    expect(await sent.device.records()).toEqual({ unsynced: 0, rejected: 0 })
  })
})
