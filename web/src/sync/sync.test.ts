// First, so Dexie finds an IndexedDB when it loads: liveQuery does nothing without one.
import 'fake-indexeddb/auto'
import { IDBFactory, IDBKeyRange } from 'fake-indexeddb'
import type { Observable } from 'dexie'
import createClient from 'openapi-fetch'
import { describe, expect, test } from 'vitest'
import type { paths } from '../api/schema.gen'
import { liveHome, openReplica } from '../local'
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
})
