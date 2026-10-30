import { useSyncExternalStore } from 'react'
import { api } from '../api/client'
import { isGoogleFailure } from './api'
import { createSession, type KeyValueStorage, type SessionState } from './session'

// Who is signed in, for the whole app. The interface reads it with
// useSession(); every 401 the API answers signs the User out (ADR 0005).

export { type CallError, type CallResult, googleSignInUrl } from './api'
export { currencyOptions, suggestCurrency } from './currency'
export { type Device, describeDevice } from './device'
export type { SessionState, SignedOutNotice } from './session'

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

export const session = createSession({ client: api, storage, notice: takeGoogleFailure() ? 'googleFailed' : undefined })

export function useSession(): SessionState {
  return useSyncExternalStore(session.subscribe, session.getState)
}
