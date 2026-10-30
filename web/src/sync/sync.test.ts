// First, so Dexie finds an IndexedDB when it loads: liveQuery does nothing without one.
import 'fake-indexeddb/auto'
import { IDBFactory, IDBKeyRange } from 'fake-indexeddb'
import type { Observable } from 'dexie'
import createClient from 'openapi-fetch'
import { describe, expect, test, vi } from 'vitest'
import type { paths } from '../api/schema.gen'
import { liveActivity, liveDebts, liveExpenses, liveHome, liveRecords, liveSettlements, openReplica } from '../local'
import { type FakeGroup, fakeServer, type FakeServer, me } from './fakeServer'
import { createSyncEngine, type Environment } from '.'

// The browser around the engine, moved by hand: the tests decide when the
// connection changes and when a timer fires.
function fakeDevice() {
  let online = true
  let visible = true
  const listeners = new Map<string, Set<() => void>>()
  const timers: { ms: number; fire: () => void }[] = []
  const emit = (event: string) => {
    for (const listener of listeners.get(event) ?? []) listener()
  }
  const environment: Environment = {
    online: () => online,
    visible: () => visible,
    listen(event, listener) {
      const forEvent = listeners.get(event) ?? new Set()
      listeners.set(event, forEvent.add(listener))
      return () => forEvent.delete(listener)
    },
    after(ms, fire) {
      const timer = { ms, fire }
      timers.push(timer)
      return () => {
        if (timers.includes(timer)) timers.splice(timers.indexOf(timer), 1)
      }
    },
  }
  return {
    environment,
    timers,
    goOffline() {
      online = false
    },
    goOnline() {
      online = true
      emit('online')
    },
    hide() {
      visible = false
      emit('visibilitychange')
    },
    show() {
      visible = true
      emit('visibilitychange')
    },
  }
}

function setup(server: FakeServer) {
  const replica = openReplica('quitsie', { indexedDB: new IDBFactory(), IDBKeyRange })
  const api = createClient<paths>({ baseUrl: 'http://quitsie.test/api/v1', fetch: server.fetch })
  const device = fakeDevice()
  const engine = createSyncEngine({ replica, api, environment: device.environment })

  // Lets the pending retry fire and waits for it to finish. Returns how long it waited.
  async function retry(): Promise<number> {
    const { ms, fire } = device.timers.shift()!
    fire()
    await vi.waitFor(() => expect(engine.getStatus()).not.toBe('syncing'))
    return ms
  }

  return { replica, engine, device, retry }
}

// What the expense form would queue: `paid` and `owed` by Member name.
function newExpense(
  group: FakeGroup,
  { description, currency, paid, owed }: { description: string; currency: string; paid: Record<string, number>; owed: Record<string, number> },
) {
  const shares = (amounts: Record<string, number>) =>
    Object.entries(amounts).map(([name, amount]) => ({ memberId: group.member[name]!, amount }))
  return {
    ledgerId: group.id,
    description,
    date: '2026-10-27',
    currency,
    total: Object.values(paid).reduce((sum, amount) => sum + amount, 0),
    splitMethod: 'equal' as const,
    paidShares: shares(paid),
    owedShares: shares(owed),
  }
}

// The first value of a live query that meets `done`, as a screen would end up rendering it.
function until<T>(observable: Observable<T>, done: (value: T) => boolean): Promise<T> {
  return new Promise((resolve, reject) => {
    const subscription = observable.subscribe({
      next(value) {
        if (!done(value)) return
        resolve(value)
        queueMicrotask(() => subscription.unsubscribe())
      },
      error: reject,
    })
  })
}

const firstValue = <T>(observable: Observable<T>) => until(observable, () => true)

describe('pull', () => {
  test('the first sync brings the User’s Groups and their Members into the replica', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana', 'Beto'] })
    const { replica, engine } = setup(server)

    await engine.sync()

    const home = await firstValue(liveHome(replica))
    expect(home).toHaveLength(1)
    expect(home[0]?.ledger).toMatchObject({ kind: 'group', id: trip.id, name: 'Viaje a Cusco', currency: 'PEN' })
    expect(home[0]?.members.map((m) => m.name).sort()).toEqual(['Ana', 'Beto', 'Yo'])
  })

  test('a later sync asks only for what changed since its cursor, and keeps deleted Expenses to restore them', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana', 'Beto'] })
    const dinner = server.addExpense(trip, { description: 'Cena', paid: { Ana: 9000 }, owed: { Yo: 3000, Ana: 3000, Beto: 3000 } })
    const { replica, engine } = setup(server)
    await engine.sync()
    const cursor = server.version()

    server.deleteExpense(dinner)
    await engine.sync()

    expect(server.pulls).toEqual([0, cursor])
    const expenses = await firstValue(liveExpenses(replica, trip.id))
    expect(expenses).toHaveLength(1)
    expect(expenses[0]).toMatchObject({ id: dinner, description: 'Cena', total: 9000, sync: 'synced' })
    expect(expenses[0]?.deletedAt).toBeDefined()
    expect(expenses[0]?.owedShares).toEqual([
      { memberId: trip.member.Yo, amount: 3000 },
      { memberId: trip.member.Ana, amount: 3000 },
      { memberId: trip.member.Beto, amount: 3000 },
    ])
  })

  test('keeps pulling while the server says it has more', async () => {
    const server = fakeServer({ versionsPerPull: 2 })
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    for (const description of ['Taxi', 'Cena', 'Museo', 'Tren']) {
      server.addExpense(trip, { description, paid: { Ana: 1000 }, owed: { Yo: 500, Ana: 500 } })
    }
    const { replica, engine } = setup(server)

    await engine.sync()

    expect(server.pulls).toEqual([0, 2, 4])
    const expenses = await firstValue(liveExpenses(replica, trip.id))
    expect(expenses.map((e) => e.description).sort()).toEqual(['Cena', 'Museo', 'Taxi', 'Tren'])
  })

  test('a ledger’s Debts are the latest ones the server computed', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana', 'Beto'] })
    server.setDebts(trip, [
      { from: 'Yo', to: 'Ana', amount: 3000 },
      { from: 'Beto', to: 'Ana', amount: 3000 },
    ])
    const { replica, engine } = setup(server)
    await engine.sync()

    server.setDebts(trip, [{ from: 'Beto', to: 'Ana', amount: 3000 }])
    await engine.sync()

    expect(await firstValue(liveDebts(replica, trip.id))).toEqual([
      { fromMemberId: trip.member.Beto, toMemberId: trip.member.Ana, amount: 3000, currency: 'PEN' },
    ])
  })

  test('brings the ledger’s Settlements', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const cash = server.addSettlement(trip, { from: 'Yo', to: 'Ana', amount: 3000 })
    const { replica, engine } = setup(server)

    await engine.sync()

    const settlements = await firstValue(liveSettlements(replica, trip.id))
    expect(settlements).toEqual([
      expect.objectContaining({ id: cash, fromMemberId: trip.member.Yo, toMemberId: trip.member.Ana, amount: 3000, sync: 'synced' }),
    ])
  })

  test('Activity lists every change, newest first, and narrows to one ledger', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const flat = server.addGroup({ name: 'Piso', currency: 'EUR', members: ['Beto'] })
    server.addExpense(trip, { description: 'Cena', paid: { Ana: 1000 }, owed: { Yo: 500, Ana: 500 } })
    server.addSettlement(flat, { from: 'Yo', to: 'Beto', amount: 2000 })
    const { replica, engine } = setup(server)

    await engine.sync()

    const kinds = (activity: { kind: string }[]) => activity.map(({ kind }) => kind)
    expect(kinds(await firstValue(liveActivity(replica)))).toEqual([
      'settlement_created',
      'expense_created',
      'group_created',
      'group_created',
    ])
    expect(kinds(await firstValue(liveActivity(replica, trip.id)))).toEqual(['expense_created', 'group_created'])
  })
})

describe('dropping ledgers', () => {
  test('a ledger the server reports as dropped leaves the replica with everything in it', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const flat = server.addGroup({ name: 'Piso', currency: 'EUR', members: ['Beto'] })
    server.addExpense(trip, { description: 'Cena', paid: { Ana: 1000 }, owed: { Yo: 500, Ana: 500 } })
    server.setDebts(trip, [{ from: 'Yo', to: 'Ana', amount: 500 }])
    const { replica, engine } = setup(server)
    await engine.sync()

    server.removeMe(trip, { reason: 'removed' })
    await engine.sync()

    const home = await firstValue(liveHome(replica))
    expect(home.map(({ ledger }) => ledger.id)).toEqual([flat.id])
    expect(await firstValue(liveExpenses(replica, trip.id))).toEqual([])
    expect(await firstValue(liveDebts(replica, trip.id))).toEqual([])
    expect((await firstValue(liveActivity(replica))).map(({ ledgerId }) => ledgerId)).toEqual([flat.id])
  })

  test('a ledger missing from visibleLedgerIds leaves the replica even if no pull reported it dropped', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const flat = server.addGroup({ name: 'Piso', currency: 'EUR', members: ['Beto'] })
    const { replica, engine } = setup(server)
    await engine.sync()

    server.removeMe(trip, { reason: 'unclaimed', announce: false })
    await engine.sync()

    const home = await firstValue(liveHome(replica))
    expect(home.map(({ ledger }) => ledger.id)).toEqual([flat.id])
  })
})

describe('410', () => {
  test('when the server restored an older backup, the replica is rebuilt from scratch', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    server.addExpense(trip, { description: 'Taxi', paid: { Ana: 1000 }, owed: { Yo: 500, Ana: 500 } })
    const backup = server.backup()
    server.addExpense(trip, { description: 'Cena', paid: { Ana: 1000 }, owed: { Yo: 500, Ana: 500 } })
    server.addExpense(trip, { description: 'Museo', paid: { Ana: 1000 }, owed: { Yo: 500, Ana: 500 } })
    const { replica, engine } = setup(server)
    await engine.sync()
    const cursor = server.version()

    server.restore(backup)
    server.addExpense(trip, { description: 'Tren', paid: { Ana: 1000 }, owed: { Yo: 500, Ana: 500 } })
    await engine.sync()

    expect(server.pulls).toEqual([0, cursor, 0])
    const expenses = await firstValue(liveExpenses(replica, trip.id))
    expect(expenses.map((e) => e.description).sort()).toEqual(['Taxi', 'Tren'])
  })
})

describe('queue', () => {
  test('an Expense created on the device waits in the queue and already shows in its ledger', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const { replica, engine, device } = setup(server)
    await engine.sync()

    device.goOffline()
    const id = await engine.addExpense(newExpense(trip, { description: 'Taxi', currency: 'PEN', paid: { Yo: 2000 }, owed: { Yo: 1000, Ana: 1000 } }))

    const records = await firstValue(liveRecords(replica))
    expect(records.rejected).toEqual([])
    expect(records.unsynced).toEqual([expect.objectContaining({ id, kind: 'expense', ledgerId: trip.id, status: 'unsynced' })])
    const expenses = await firstValue(liveExpenses(replica, trip.id))
    expect(expenses).toEqual([expect.objectContaining({ id, description: 'Taxi', total: 2000, sync: 'unsynced' })])
  })

  test('a sync sends the queue: an accepted Expense leaves the inbox and comes back from the server', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const { replica, engine, device } = setup(server)
    device.goOffline()
    const id = await engine.addExpense(newExpense(trip, { description: 'Taxi', currency: 'PEN', paid: { Yo: 2000 }, owed: { Yo: 1000, Ana: 1000 } }))

    device.goOnline()
    await engine.sync()

    expect(await firstValue(liveRecords(replica))).toEqual({ rejected: [], unsynced: [] })
    expect(await firstValue(liveExpenses(replica, trip.id))).toEqual([
      expect.objectContaining({ id, description: 'Taxi', createdBy: me, sync: 'synced' }),
    ])
  })

  test('a rejected Expense stays in the inbox with its reason, never dropped', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const { replica, engine, device } = setup(server)
    await engine.sync()
    device.goOffline()
    const id = await engine.addExpense(newExpense(trip, { description: 'Taxi', currency: 'PEN', paid: { Yo: 2000 }, owed: { Yo: 1000, Ana: 1000 } }))

    server.setCurrency(trip, 'USD')
    device.goOnline()
    await engine.sync()

    const records = await firstValue(liveRecords(replica))
    expect(records.unsynced).toEqual([])
    expect(records.rejected).toEqual([expect.objectContaining({ id, status: 'rejected', reason: 'currency_mismatch' })])
    expect(await firstValue(liveExpenses(replica, trip.id))).toEqual([expect.objectContaining({ id, sync: 'rejected' })])
  })
})

describe('retries', () => {
  test('a failed sync retries after a growing wait, and starts over once the server answers', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const { replica, engine, device, retry } = setup(server)
    server.setReachable(false)

    await engine.sync()

    expect(engine.getStatus()).toBe('retrying')
    expect(await retry()).toBe(1000)
    expect(await retry()).toBe(2000)
    expect(await retry()).toBe(4000)

    server.setReachable(true)
    expect(await retry()).toBe(8000)

    expect(engine.getStatus()).toBe('idle')
    expect(device.timers).toEqual([])
    expect((await firstValue(liveHome(replica))).map(({ ledger }) => ledger.id)).toEqual([trip.id])

    server.setReachable(false)
    await engine.sync()
    expect(device.timers.map(({ ms }) => ms)).toEqual([1000])
  })

  test('the wait stops growing at five minutes', async () => {
    const server = fakeServer()
    const { engine, retry } = setup(server)
    server.setReachable(false)
    await engine.sync()

    const waits: number[] = []
    for (let i = 0; i < 11; i++) waits.push(await retry())

    expect(waits).toEqual([1000, 2000, 4000, 8000, 16000, 32000, 64000, 128000, 256000, 300000, 300000])
  })

  test('a server error retries too', async () => {
    const server = fakeServer()
    const { engine, device } = setup(server)
    server.failWith(503)

    await engine.sync()

    expect(engine.getStatus()).toBe('retrying')
    expect(device.timers.map(({ ms }) => ms)).toEqual([1000])
  })

  test('with the session gone it stops retrying until something else starts a sync', async () => {
    const server = fakeServer()
    const { engine, device } = setup(server)
    server.failWith(401)

    await engine.sync()

    expect(engine.getStatus()).toBe('signedOut')
    expect(device.timers).toEqual([])
  })
})

describe('triggers', () => {
  test('an Expense created offline is accepted after reconnecting', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const { replica, engine, device } = setup(server)
    engine.start()
    await until(liveHome(replica), (home) => home.length === 1)

    device.goOffline()
    server.setReachable(false)
    const id = await engine.addExpense(newExpense(trip, { description: 'Taxi', currency: 'PEN', paid: { Yo: 2000 }, owed: { Yo: 1000, Ana: 1000 } }))

    expect(engine.getStatus()).toBe('offline')
    expect(server.pushes).toEqual([])
    expect(await firstValue(liveExpenses(replica, trip.id))).toEqual([expect.objectContaining({ id, sync: 'unsynced' })])

    server.setReachable(true)
    device.goOnline()

    const [expense] = await until(liveExpenses(replica, trip.id), ([first]) => first?.version !== undefined)
    expect(expense).toMatchObject({ id, sync: 'synced', createdBy: me })
    expect(await firstValue(liveRecords(replica))).toEqual({ rejected: [], unsynced: [] })
    expect(engine.getStatus()).toBe('idle')
  })

  test('opening the app syncs', async () => {
    const server = fakeServer()
    server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const { replica, engine } = setup(server)

    engine.start()

    expect(await until(liveHome(replica), (home) => home.length === 1)).toHaveLength(1)
  })

  test('coming back to the app syncs; leaving it does not', async () => {
    const server = fakeServer()
    const { replica, engine, device } = setup(server)
    await engine.sync()
    engine.start()
    await vi.waitFor(() => expect(engine.getStatus()).toBe('idle'))

    device.hide()
    expect(engine.getStatus()).toBe('idle')

    server.addGroup({ name: 'Piso', currency: 'EUR', members: ['Beto'] })
    device.show()

    expect(await until(liveHome(replica), (home) => home.length === 1)).toHaveLength(1)
  })

  test('a record created online is sent right away', async () => {
    const server = fakeServer()
    const trip = server.addGroup({ name: 'Viaje a Cusco', currency: 'PEN', members: ['Ana'] })
    const { replica, engine } = setup(server)
    await engine.sync()

    const id = await engine.addExpense(newExpense(trip, { description: 'Taxi', currency: 'PEN', paid: { Yo: 2000 }, owed: { Yo: 1000, Ana: 1000 } }))

    const [expense] = await until(liveExpenses(replica, trip.id), ([first]) => first?.version !== undefined)
    expect(expense).toMatchObject({ id, sync: 'synced' })
  })

  test('after stop, nothing starts a sync', async () => {
    const server = fakeServer()
    const { engine, device } = setup(server)
    server.setReachable(false)
    engine.start()
    await vi.waitFor(() => expect(device.timers).toHaveLength(1))

    engine.stop()

    expect(device.timers).toEqual([])
    device.goOffline()
    device.goOnline()
    device.show()
    expect(engine.getStatus()).toBe('retrying')
  })

  test('only one sync runs at a time, and those asked for meanwhile become one more', async () => {
    const server = fakeServer()
    const { engine } = setup(server)

    await Promise.all([engine.sync(), engine.sync(), engine.sync(), engine.sync()])

    expect(server.busiest()).toBe(1)
    expect(server.pulls).toHaveLength(2)
  })

  test('the status tells a listener what the engine is doing', async () => {
    const server = fakeServer()
    const { engine } = setup(server)
    const seen: string[] = []
    engine.subscribe(() => seen.push(engine.getStatus()))

    await engine.sync()

    expect(seen).toEqual(['syncing', 'idle'])
  })
})
