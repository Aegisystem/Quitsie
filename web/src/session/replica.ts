import { clearDevice, type Replica } from '../local'
import type { SyncEngine } from '../sync'
import type { DeviceData, KeyValueStorage } from './session'

// Whose data the device holds. A session that ended by itself (expired, closed
// from another device) leaves the replica and the queue in place, so the same
// User carries on after signing in again; anyone else starts from zero.
const OWNER_KEY = 'quitsie.replica.owner'

// A pull still on its way would write after the wipe.
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
  // If the wipe fails the owner stays, and the next sign-in tries again.
  async function wipe(): Promise<boolean> {
    engine.stop()
    await finished(engine)
    try {
      await clearDevice(replica)
      storage.removeItem(OWNER_KEY)
      return true
    } catch {
      return false
    }
  }

  return {
    async admit(userId) {
      if (storage.getItem(OWNER_KEY) === userId) return
      if (await wipe()) storage.setItem(OWNER_KEY, userId)
    },

    async clear() {
      await wipe()
    },

    async records() {
      await engine.sync()
      const [unsynced, rejected] = await Promise.all([replica.queue.where({ status: 'unsynced' }).count(), replica.queue.where({ status: 'rejected' }).count()])
      return { unsynced, rejected }
    },
  }
}
