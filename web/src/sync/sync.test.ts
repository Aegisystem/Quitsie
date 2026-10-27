// First, so Dexie finds an IndexedDB when it loads: liveQuery does nothing without one.
import 'fake-indexeddb/auto'
import { IDBFactory, IDBKeyRange } from 'fake-indexeddb'
import type { Observable } from 'dexie'
import createClient from 'openapi-fetch'
import { describe, expect, test } from 'vitest'
import type { paths } from '../api/schema.gen'
import { liveActivity, liveDebts, liveExpenses, liveHome, liveSettlements, openReplica } from '../local'
import { fakeServer, type FakeServer } from './fakeServer'
import { createSyncEngine } from '.'

function setup(server: FakeServer) {
  const replica = openReplica('quitsie', { indexedDB: new IDBFactory(), IDBKeyRange })
  const api = createClient<paths>({ baseUrl: 'http://quitsie.test/api/v1', fetch: server.fetch })
  const engine = createSyncEngine({ replica, api })
  return { replica, engine }
}

// The first value a live query emits, as a screen would render it.
function firstValue<T>(observable: Observable<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    const subscription = observable.subscribe({
      next(value) {
        resolve(value)
        queueMicrotask(() => subscription.unsubscribe())
      },
      error: reject,
    })
  })
}

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
