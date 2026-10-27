import createClient from 'openapi-fetch'
import { describe, expect, it } from 'vitest'
import type { Session, User } from '../api/client'
import type { paths } from '../api/schema.gen'
import { googleSignInUrl, isGoogleFailure } from './api'
import { createSession, type KeyValueStorage, type SessionOptions } from './session'

const ana: User = { id: '0199f0a0-0000-7000-8000-000000000001', email: 'ana@example.com', name: 'Ana', preferredCurrency: 'MXN' }
const newcomer: User = { ...ana, name: '', preferredCurrency: 'USD' }

// A fake server behind a simulated fetch: each route answers with a function,
// and a route that throws is a network failure.
type Handler = (request: Request) => Response | Promise<Response>

function fakeServer(routes: Record<string, Handler>) {
  const requests: Request[] = []
  const fetch = async (request: Request) => {
    requests.push(request.clone())
    const path = new URL(request.url).pathname.replace('/api/v1', '')
    const handler = routes[`${request.method} ${path}`] ?? (() => problem(404, 'not_found'))
    return handler(request)
  }
  return { requests, client: createClient<paths>({ baseUrl: 'http://quitsie.test/api/v1', fetch }) }
}

const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const noContent = () => new Response(null, { status: 204 })
const problem = (status: number, code: string, headers: Record<string, string> = {}) =>
  new Response(JSON.stringify({ title: code, status, code }), { status, headers: { 'Content-Type': 'application/problem+json', ...headers } })
const unreachable: Handler = () => {
  throw new TypeError('Failed to fetch')
}

function memoryStorage(user?: User): KeyValueStorage & { data: Map<string, string> } {
  const data = new Map<string, string>(user ? [['quitsie.session.user', JSON.stringify(user)]] : [])
  return {
    data,
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => void data.set(key, value),
    removeItem: (key) => void data.delete(key),
  }
}

function setup(routes: Record<string, Handler>, opts: { remembered?: User; notice?: SessionOptions['notice'] } = {}) {
  const server = fakeServer(routes)
  const storage = memoryStorage(opts.remembered)
  const session = createSession({ client: server.client, storage, notice: opts.notice })
  return { ...server, storage, session }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((r) => (resolve = r))
  return { promise, resolve }
}

describe('opening the app', () => {
  it('asks /me and signs in with the User it returns', async () => {
    const { session, storage } = setup({ 'GET /me': () => json(200, ana) })
    expect(session.getState()).toEqual({ status: 'loading' })
    await session.check()
    expect(session.getState()).toEqual({ status: 'signedIn', user: ana, needsProfile: false })
    expect(storage.data.has('quitsie.session.user')).toBe(true)
  })

  it('counts a 401 as signed out', async () => {
    const { session } = setup({ 'GET /me': () => problem(401, 'unauthenticated') })
    await session.check()
    expect(session.getState()).toEqual({ status: 'signedOut' })
  })

  it('opens signed in with the remembered User and keeps it while the server cannot be reached', async () => {
    const { session } = setup({ 'GET /me': unreachable }, { remembered: ana })
    expect(session.getState()).toEqual({ status: 'signedIn', user: ana, needsProfile: false })
    await session.check()
    expect(session.getState()).toEqual({ status: 'signedIn', user: ana, needsProfile: false })
  })

  it('keeps the remembered User when the server fails', async () => {
    const { session } = setup({ 'GET /me': () => problem(501, 'not_implemented') }, { remembered: ana })
    await session.check()
    expect(session.getState()).toMatchObject({ status: 'signedIn', user: ana })
  })

  it('updates the remembered User with what /me says now', async () => {
    const renamed = { ...ana, name: 'Ana María' }
    const { session, storage } = setup({ 'GET /me': () => json(200, renamed) }, { remembered: ana })
    await session.check()
    expect(session.getState()).toMatchObject({ user: renamed })
    expect(JSON.parse(storage.data.get('quitsie.session.user') ?? '')).toEqual(renamed)
  })

  it('signs out a remembered User whose session ended, and forgets them', async () => {
    const { session, storage } = setup({ 'GET /me': () => problem(401, 'unauthenticated') }, { remembered: ana })
    await session.check()
    expect(session.getState()).toEqual({ status: 'signedOut', notice: 'sessionEnded' })
    expect(storage.data.size).toBe(0)
  })

  it('shows the sign-in screen when nobody is remembered and the server cannot be reached', async () => {
    const { session } = setup({ 'GET /me': unreachable })
    await session.check()
    expect(session.getState()).toEqual({ status: 'signedOut' })
  })

  it('tells why after a failed Google sign-in', async () => {
    const { session } = setup({ 'GET /me': () => problem(401, 'unauthenticated') }, { notice: 'googleFailed' })
    await session.check()
    expect(session.getState()).toEqual({ status: 'signedOut', notice: 'googleFailed' })
  })

  it('asks for a profile when the User has no name yet', async () => {
    const { session } = setup({ 'GET /me': () => json(200, newcomer) })
    await session.check()
    expect(session.getState()).toMatchObject({ status: 'signedIn', needsProfile: true })
  })

  it('ignores a remembered User it cannot read', async () => {
    const server = fakeServer({})
    const storage = memoryStorage()
    storage.setItem('quitsie.session.user', '{"id":1')
    expect(createSession({ client: server.client, storage }).getState()).toEqual({ status: 'loading' })
  })

  it('sends a single /me while a check is already running', async () => {
    const { session, requests } = setup({ 'GET /me': () => json(200, ana) })
    await Promise.all([session.check(), session.check()])
    expect(requests).toHaveLength(1)
  })
})

describe('signing in with a code', () => {
  it('requests a code for the email', async () => {
    const { session, requests } = setup({ 'POST /auth/login-code': noContent })
    expect(await session.requestCode('ana@example.com')).toEqual({ ok: true, value: undefined })
    expect(await requests[0]?.json()).toEqual({ email: 'ana@example.com' })
  })

  it('reports how long to wait when too many codes were requested', async () => {
    const { session } = setup({ 'POST /auth/login-code': () => problem(429, 'rate_limited', { 'Retry-After': '600' }) })
    expect(await session.requestCode('ana@example.com')).toEqual({
      ok: false,
      error: { kind: 'problem', status: 429, code: 'rate_limited', retryAfter: 600 },
    })
  })

  it('reports a failure with no answer as offline', async () => {
    const { session } = setup({ 'POST /auth/login-code': unreachable })
    expect(await session.requestCode('ana@example.com')).toEqual({ ok: false, error: { kind: 'offline' } })
  })

  it('reads the status of an error that is not a problem', async () => {
    const { session } = setup({ 'POST /auth/login-code': () => new Response('Forbidden', { status: 403 }) })
    expect(await session.requestCode('ana@example.com')).toEqual({
      ok: false,
      error: { kind: 'problem', status: 403, code: undefined, retryAfter: undefined },
    })
  })

  it('signs in with the right code', async () => {
    const { session, requests } = setup({ 'POST /auth/login-code/verify': () => json(200, { user: ana, newUser: false }) })
    expect(await session.verifyCode('ana@example.com', '042917')).toEqual({ ok: true, value: undefined })
    expect(await requests[0]?.json()).toEqual({ email: 'ana@example.com', code: '042917' })
    expect(session.getState()).toEqual({ status: 'signedIn', user: ana, needsProfile: false })
  })

  it('asks a new User for a profile', async () => {
    const { session } = setup({ 'POST /auth/login-code/verify': () => json(200, { user: newcomer, newUser: true }) })
    await session.verifyCode('ana@example.com', '042917')
    expect(session.getState()).toMatchObject({ status: 'signedIn', user: newcomer, needsProfile: true })
  })

  it('stays signed out with a wrong code and says why', async () => {
    const { session } = setup({ 'POST /auth/login-code/verify': () => problem(400, 'login_code_invalid') })
    const result = await session.verifyCode('ana@example.com', '000000')
    expect(result).toMatchObject({ ok: false, error: { kind: 'problem', code: 'login_code_invalid' } })
    expect(session.getState().status).not.toBe('signedIn')
  })

  it('does not let an older 401 from /me undo a sign-in', async () => {
    const me = deferred<Response>()
    const { session } = setup({
      'GET /me': () => me.promise,
      'POST /auth/login-code/verify': () => json(200, { user: ana, newUser: false }),
    })
    const checking = session.check()
    await session.verifyCode('ana@example.com', '042917')
    me.resolve(problem(401, 'unauthenticated'))
    await checking
    expect(session.getState()).toMatchObject({ status: 'signedIn', user: ana })
  })
})

describe('the profile', () => {
  it('saves the name and Preferred currency and stops asking for them', async () => {
    const saved = { ...newcomer, name: 'Ana', preferredCurrency: 'COP' }
    const { session, requests, storage } = setup({
      'POST /auth/login-code/verify': () => json(200, { user: newcomer, newUser: true }),
      'PATCH /me': () => json(200, saved),
    })
    await session.verifyCode('ana@example.com', '042917')
    expect(await session.saveProfile({ name: 'Ana', preferredCurrency: 'COP' })).toEqual({ ok: true, value: saved })
    expect(await requests[1]?.json()).toEqual({ name: 'Ana', preferredCurrency: 'COP' })
    expect(session.getState()).toEqual({ status: 'signedIn', user: saved, needsProfile: false })
    expect(JSON.parse(storage.data.get('quitsie.session.user') ?? '')).toEqual(saved)
  })

  it('keeps asking when saving fails', async () => {
    const { session } = setup({ 'GET /me': () => json(200, newcomer), 'PATCH /me': () => problem(400, 'validation_failed') }, { remembered: newcomer })
    const result = await session.saveProfile({ name: 'x'.repeat(81) })
    expect(result).toMatchObject({ ok: false, error: { code: 'validation_failed' } })
    expect(session.getState()).toMatchObject({ needsProfile: true })
  })
})

describe('signing out', () => {
  it('closes the session and forgets the User', async () => {
    const { session, storage, requests } = setup({ 'POST /auth/logout': noContent }, { remembered: ana })
    expect(await session.signOut()).toEqual({ ok: true, value: undefined })
    expect(requests[0]?.url).toBe('http://quitsie.test/api/v1/auth/logout')
    expect(session.getState()).toEqual({ status: 'signedOut' })
    expect(storage.data.size).toBe(0)
  })

  it('stays signed in when the server cannot be reached, because only it can clear the cookie', async () => {
    const { session } = setup({ 'POST /auth/logout': unreachable }, { remembered: ana })
    expect(await session.signOut()).toEqual({ ok: false, error: { kind: 'offline' } })
    expect(session.getState()).toMatchObject({ status: 'signedIn' })
  })

  it('deletes the account and says so on the sign-in screen', async () => {
    const { session, requests } = setup({ 'DELETE /me': noContent }, { remembered: ana })
    expect(await session.deleteAccount()).toEqual({ ok: true, value: undefined })
    expect(requests[0]?.method).toBe('DELETE')
    expect(session.getState()).toEqual({ status: 'signedOut', notice: 'accountDeleted' })
  })

  it('keeps the account when deleting it fails', async () => {
    const { session } = setup({ 'DELETE /me': () => problem(500, 'internal_error') }, { remembered: ana })
    expect(await session.deleteAccount()).toMatchObject({ ok: false, error: { status: 500 } })
    expect(session.getState()).toMatchObject({ status: 'signedIn' })
  })

  it('signs out on any 401, such as one from the sync engine', async () => {
    const { session, client } = setup({ 'GET /sync': () => problem(401, 'unauthenticated') }, { remembered: ana })
    await client.GET('/sync', { params: { query: { since: 0 } } })
    expect(session.getState()).toEqual({ status: 'signedOut', notice: 'sessionEnded' })
  })
})

describe('sessions on each device', () => {
  const here: Session = { id: '0199f0a0-0000-7000-8000-0000000000a1', device: 'Mozilla/5.0 (iPhone)', createdAt: '2026-10-01T10:00:00Z', lastUsedAt: '2026-10-27T09:00:00Z', current: true }
  const laptop: Session = { ...here, id: '0199f0a0-0000-7000-8000-0000000000a2', current: false }

  it('lists them', async () => {
    const { session } = setup({ 'GET /auth/sessions': () => json(200, [here, laptop]) }, { remembered: ana })
    expect(await session.listSessions()).toEqual({ ok: true, value: [here, laptop] })
  })

  it('closes another device and stays signed in', async () => {
    const { session, requests } = setup({ [`DELETE /auth/sessions/${laptop.id}`]: noContent }, { remembered: ana })
    expect(await session.revokeSession(laptop)).toEqual({ ok: true, value: undefined })
    expect(requests).toHaveLength(1)
    expect(session.getState()).toMatchObject({ status: 'signedIn' })
  })

  it('closing this device signs out', async () => {
    const { session } = setup({ [`DELETE /auth/sessions/${here.id}`]: noContent }, { remembered: ana })
    await session.revokeSession(here)
    expect(session.getState()).toEqual({ status: 'signedOut' })
  })
})

describe('Google', () => {
  it('starts at the API and comes back to the route the User was on', () => {
    expect(googleSignInUrl()).toBe('/api/v1/auth/google')
    expect(googleSignInUrl('/#/invite/lisboa-7Qx2')).toBe('/api/v1/auth/google?returnTo=%2F%23%2Finvite%2Flisboa-7Qx2')
  })

  it('recognizes the redirect after a failed sign-in', () => {
    expect(isGoogleFailure(new URL('https://quitsie.app/login?error=google'))).toBe(true)
    expect(isGoogleFailure(new URL('https://quitsie.app/'))).toBe(false)
    expect(isGoogleFailure(new URL('https://quitsie.app/login'))).toBe(false)
  })
})
