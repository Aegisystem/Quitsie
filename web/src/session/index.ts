import { useSyncExternalStore } from 'react'
import { api } from '../api/client'
import { openReplica } from '../local'
import { diagnostics } from '../diagnostics'
import { browserPush, createDevicePush } from '../push/device'
import { createSyncEngine } from '../sync'
import { isGoogleFailure } from './api'
import { createLedgerCalls } from './ledgers'
import { createDeviceData } from './replica'
import { createSession, type KeyValueStorage, type SessionState } from './session'

// Who is signed in, for the whole app. The interface reads it with
// useSession(); every 401 the API answers signs the User out (ADR 0005).

export { type CallError, type CallResult, googleSignInUrl, type LossyRecords } from './api'
export { suggestCurrency } from './currency'
export { type Device, describeDevice } from './device'
export type { GroupUpdate, InviteLinkOwner, NewGroup, NewPendingMember } from './ledgers'
export type { DeviceRecords, SessionState, SignedOutNotice } from './session'
export type { PushStatus } from '../push/device'

// Private mode or blocked storage: the User is just not remembered offline.
const storage: KeyValueStorage = {
  getItem(key) {
    try {
      return localStorage.getItem(key)
    } catch {
      return null
    }
  },
  setItem(key, value) {
    try {
      localStorage.setItem(key, value)
    } catch {
      // Not remembered.
    }
  },
  removeItem(key) {
    try {
      localStorage.removeItem(key)
    } catch {
      // Nothing was remembered.
    }
  },
}

// The server sends a failed Google sign-in to /login?error=google and a
// cancelled one to /login. Read it once and put the address back to the app's
// root, keeping the route.
function takeGoogleFailure(): boolean {
  const url = new URL(location.href)
  if (url.pathname !== '/login') return false
  history.replaceState(null, '', `/${location.hash}`)
  return isGoogleFailure(url)
}

// The replica and the sync engine belong to whoever is signed in, so they are
// created here, with the `api` whose 401s sign the User out.
export const replica = openReplica()
export const engine = createSyncEngine({ replica, api, report: (message) => diagnostics.report('sync', message) })

export const session = createSession({
  client: api,
  storage,
  notice: takeGoogleFailure() ? 'googleFailed' : undefined,
  device: createDeviceData({ replica, engine, storage }),
})

export const ledgers = createLedgerCalls({ client: api, sync: () => engine.sync() })

// Push notifications belong to the session too: each sign-in sends this
// device's subscription again, and signing out drops it.
export const pushNotices = createDevicePush({ client: api, browser: browserPush() })

// The engine runs while someone is signed in.
let running = false
function followSession() {
  const signedIn = session.getState().status === 'signedIn'
  if (signedIn === running) return
  running = signedIn
  if (signedIn) {
    engine.start()
    void pushNotices.refresh()
  } else {
    engine.stop()
    void pushNotices.forget()
  }
}
session.subscribe(followSession)
followSession()

// Signing out or deleting the account leaves nothing of Quitsie behind, the
// diagnostics log included. When the session ends by itself the log stays:
// that may be part of the problem it records.
session.subscribe(() => {
  const state = session.getState()
  if (state.status === 'signedOut' && (state.notice === undefined || state.notice === 'accountDeleted')) diagnostics.clear()
})

export function useSession(): SessionState {
  return useSyncExternalStore(session.subscribe, session.getState)
}

