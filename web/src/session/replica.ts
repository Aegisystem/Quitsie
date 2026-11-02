import { clearDevice, handOver, ownerOf, releaseReplica, type Replica } from '../local'
import type { SyncEngine } from '../sync'
import type { DeviceData, KeyValueStorage } from './session'

// Whose data the device holds is kept in the replica itself, so that handing
// the device over is one transaction. A session that ended by itself (expired,
// closed from another device, account deleted) leaves only the queue, the one
// thing the server lacks: the same User sends it after signing in again, and
// anyone else finds nothing of it (docs/reglas-de-negocio.md, "Sin conexión").

// Where the owner was kept before, read once if the replica does not say.
const LEGACY_OWNER_KEY = 'quitsie.replica.owner'

// A pull still on its way would write after the change.
function finished(engine: SyncEngine): Promise<void> {
  return new Promise((resolve) => {
    if (engine.getStatus() !== 'syncing') return resolve()
    const stop = engine.subscribe(() => {
      if (engine.getStatus() === 'syncing') return
      stop()
      resolve()
    })
  })
}

export function createDeviceData({ replica, engine, storage }: { replica: Replica; engine: SyncEngine; storage: KeyValueStorage }): DeviceData {
  // If a change fails, what was there stays, and the next one tries again.
  async function quietly(change: () => Promise<void>): Promise<void> {
    engine.stop()
    await finished(engine)
    try {
      await change()
    } catch {
      // Tried again later.
    }
  }

  return {
    async admit(userId) {
      const owner = await ownerOf(replica).catch(() => undefined)
      if (owner === userId) return
      await quietly(async () => {
        await handOver(replica, userId, storage.getItem(LEGACY_OWNER_KEY) ?? undefined)
        storage.removeItem(LEGACY_OWNER_KEY)
      })
    },

    // It never touches the queue, so a sign-in that overtakes it loses nothing.
    release: () => quietly(() => releaseReplica(replica)),

    clear: () => quietly(() => clearDevice(replica)),

    async records() {
      await engine.sync()
      const [unsynced, rejected] = await Promise.all([replica.queue.where({ status: 'unsynced' }).count(), replica.queue.where({ status: 'rejected' }).count()])
      return { unsynced, rejected }
    },
  }
}
