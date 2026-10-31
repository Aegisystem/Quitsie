import { type Observable } from 'dexie'
import { useEffect, useState, useSyncExternalStore } from 'react'
import { api } from '../api/client'
import { openReplica } from '../local'
import { createSyncEngine } from '../sync'
import { isGoogleFailure } from './api'
import { createLedgerCalls } from './ledgers'
import { createDeviceData } from './replica'
import { createSession, type KeyValueStorage, type SessionState } from './session'

// Who is signed in, for the whole app. The interface reads it with
// useSession(); every 401 the API answers signs the User out (ADR 0005).

export { type CallError, type CallResult, googleSignInUrl, type LossyRecords } from './api'
export { currencyOptions, suggestCurrency } from './currency'
export { type Device, describeDevice } from './device'
export type { GroupUpdate, InviteLinkOwner, NewGroup, NewPendingMember } from './ledgers'
export type { DeviceRecords, SessionState, SignedOutNotice } from './session'

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
export const engine = createSyncEngine({ replica, api })

export const session = createSession({
  client: api,
  storage,
  notice: takeGoogleFailure() ? 'googleFailed' : undefined,
  device: createDeviceData({ replica, engine, storage }),
})

export const ledgers = createLedgerCalls({ client: api, sync: () => engine.sync() })

// The engine runs while someone is signed in.
let running = false
function followSession() {
  const signedIn = session.getState().status === 'signedIn'
  if (signedIn === running) return
  running = signedIn
  if (signedIn) engine.start()
  else engine.stop()
}
session.subscribe(followSession)
followSession()

export function useSession(): SessionState {
  return useSyncExternalStore(session.subscribe, session.getState)
}

// Renders every value of a live query of ../local. `key` names the query, and
// `loaded` tells one that has not answered yet from one that answered undefined.
export function useLive<T>(key: string, query: () => Observable<T>): [value: T | undefined, loaded: boolean] {
  const [latest, setLatest] = useState<{ key: string; value: T }>()
  useEffect(() => {
    const subscription = query().subscribe((value) => setLatest({ key, value }))
    return () => subscription.unsubscribe()
    // The key stands for the query, which is a new function on every render.
  }, [key])
  return latest?.key === key ? [latest.value, true] : [undefined, false]
}
