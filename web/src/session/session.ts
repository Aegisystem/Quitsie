import type { LoginMethods, Session, User } from '../api/client'
import { ok } from '../domain/result'
import * as calls from './api'
import type { ApiClient, CallResult, UserUpdate } from './api'

// Why the User is looking at the sign-in screen, when it is not obvious.
export type SignedOutNotice = 'googleFailed' | 'sessionEnded' | 'accountDeleted'

export type SessionState =
  | { status: 'loading' }
  | { status: 'signedOut'; notice?: SignedOutNotice }
  // needsProfile: the User is new or has no name yet, so the app asks for a
  // name and a Preferred currency before anything else.
  | { status: 'signedIn'; user: User; needsProfile: boolean }

export type KeyValueStorage = Pick<Storage, 'getItem' | 'setItem' | 'removeItem'>

// Records the server does not have: signing out loses them.
export interface DeviceRecords {
  unsynced: number
  rejected: number
}

// What the device keeps between sessions: the replica and the queue.
export interface DeviceData {
  // Before `userId` sees the app: what another User left here goes.
  admit(userId: string): Promise<void>
  // The User left on purpose, so nothing of theirs stays.
  clear(): Promise<void>
  // After trying to send them, so only those that could not be sent count.
  records(): Promise<DeviceRecords>
}

const nothingOnDevice: DeviceData = {
  admit: async () => {},
  clear: async () => {},
  records: async () => ({ unsynced: 0, rejected: 0 }),
}

export interface SessionOptions {
  client: ApiClient
  // Keeps the last known User so the app opens signed in without a network.
  storage: KeyValueStorage
  // Shown if the first check of /me finds no session.
  notice?: SignedOutNotice
  device?: DeviceData
}

const USER_KEY = 'quitsie.session.user'

export type SessionStore = ReturnType<typeof createSession>

export function createSession({ client, storage, notice: firstNotice, device = nothingOnDevice }: SessionOptions) {
  const cached = readUser(storage)
  let state: SessionState = cached ? signedIn(cached, false) : { status: 'loading' }
  // Bumped on every sign-in, sign-out and saved profile, so an answer to a
  // request sent before it can never undo it.
  let generation = 0
  const listeners = new Set<() => void>()

  function set(next: SessionState) {
    state = next
    for (const l of listeners) l()
  }

  function enter(user: User, newUser: boolean) {
    generation++
    writeUser(storage, user)
    set(signedIn(user, newUser))
  }

  function leave(notice?: SignedOutNotice) {
    generation++
    storage.removeItem(USER_KEY)
    set({ status: 'signedOut', notice })
  }

  // Any 401, from these calls or from the sync engine's, means the session is
  // gone: expired, closed from another device or the account was deleted.
  const sentIn = new WeakMap<Request, number>()
  client.use({
    onRequest({ request }) {
      sentIn.set(request, generation)
    },
    onResponse({ request, response }) {
      if (response.status === 401 && state.status === 'signedIn' && (sentIn.get(request) ?? generation) === generation) {
        leave('sessionEnded')
      }
    },
  })

  let checking: Promise<void> | undefined

  // Asks the server who is signed in. Without an answer (offline, or the server
  // failing) the cached User stays, so the app keeps working offline.
  async function check() {
    const sent = generation
    const me = await calls.getMe(client)
    if (sent !== generation) return
    if (me.ok) {
      if (state.status !== 'signedIn') {
        await device.admit(me.value.id)
        if (sent !== generation) return
        return enter(me.value, false)
      }
      writeUser(storage, me.value)
      set(signedIn(me.value, state.needsProfile))
    } else if (me.error.kind === 'problem' && me.error.status === 401) {
      if (state.status !== 'signedOut') leave(state.status === 'loading' ? firstNotice : 'sessionEnded')
    } else if (state.status === 'loading') {
      // Nobody to fall back on, and signing in needs the network anyway.
      set({ status: 'signedOut', notice: firstNotice })
    }
  }

  async function settle(result: CallResult<void>, notice?: SignedOutNotice): Promise<CallResult<void>> {
    // A 401 means there was no session to close: the User is signed out either way.
    if (result.ok || (result.error.kind === 'problem' && result.error.status === 401)) {
      // A 401 from here on is about the session that just ended.
      generation++
      await device.clear()
      leave(notice)
      return ok(undefined)
    }
    return result
  }

  return {
    getState: (): SessionState => state,

    subscribe(listener: () => void) {
      listeners.add(listener)
      return () => {
        listeners.delete(listener)
      }
    },

    // Called when the app opens and when the connection comes back.
    check(): Promise<void> {
      checking ??= check().finally(() => {
        checking = undefined
      })
      return checking
    },

    // Which ways of signing in the server offers besides the code.
    loginMethods(): Promise<CallResult<LoginMethods>> {
      return calls.getLoginMethods(client)
    },

    requestCode(email: string): Promise<CallResult<void>> {
      return calls.requestLoginCode(client, email)
    },

    async verifyCode(email: string, code: string): Promise<CallResult<void>> {
      const result = await calls.verifyLoginCode(client, email, code)
      if (!result.ok) return result
      await device.admit(result.value.user.id)
      enter(result.value.user, result.value.newUser)
      return ok(undefined)
    },

    // Name and Preferred currency, from the welcome screen or the account.
    async saveProfile(update: UserUpdate): Promise<CallResult<User>> {
      const sent = generation
      const result = await calls.updateMe(client, update)
      if (result.ok && sent === generation) {
        generation++
        writeUser(storage, result.value)
        set(signedIn(result.value, false))
      }
      return result
    },

    // What signing out would lose. The screens warn with it first.
    recordsOnDevice(): Promise<DeviceRecords> {
      return device.records()
    },

    // Needs the network: the cookie is HttpOnly, so only the server can clear it.
    // Once the server has closed the session, the replica and the queue go.
    async signOut(): Promise<CallResult<void>> {
      return settle(await calls.logout(client))
    },

    async deleteAccount(): Promise<CallResult<void>> {
      const result = await calls.deleteMe(client)
      return result.ok ? settle(result, 'accountDeleted') : result
    },

    listSessions(): Promise<CallResult<Session[]>> {
      return calls.listSessions(client)
    },

    // Closing the current session is the same as signing out.
    async revokeSession(target: Session): Promise<CallResult<void>> {
      const result = await calls.revokeSession(client, target.id)
      return target.current ? settle(result) : result
    },
  }
}

function signedIn(user: User, newUser: boolean): SessionState {
  return { status: 'signedIn', user, needsProfile: newUser || user.name.trim() === '' }
}

function readUser(storage: KeyValueStorage): User | undefined {
  try {
    const raw = storage.getItem(USER_KEY)
    const user: unknown = raw ? JSON.parse(raw) : undefined
    return isUser(user) ? user : undefined
  } catch {
    return undefined
  }
}

function writeUser(storage: KeyValueStorage, user: User) {
  storage.setItem(USER_KEY, JSON.stringify(user))
}

function isUser(value: unknown): value is User {
  if (typeof value !== 'object' || value === null) return false
  const u = value as Record<string, unknown>
  return ['id', 'email', 'name', 'preferredCurrency'].every((key) => typeof u[key] === 'string')
}
