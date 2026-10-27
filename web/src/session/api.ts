import type { Client } from 'openapi-fetch'
import type { ProblemCode, Session, User } from '../api/client'
import type { components, paths } from '../api/schema.gen'
import { fail, ok, type Result } from '../domain/result'

// The auth and profile calls of openapi.yaml. The client is passed in so tests
// can give it a simulated fetch.
export type ApiClient = Client<paths>

// web/src/api/client.ts does not export these two aliases yet.
export type LoginResult = components['schemas']['LoginResult']
export type UserUpdate = components['schemas']['UserUpdate']

// Why a call failed, in the terms the screens need. `offline` means no answer
// came back at all: no network, or the server could not be reached.
export type CallError =
  | { kind: 'offline' }
  | { kind: 'problem'; status: number; code?: ProblemCode; retryAfter?: number }

export type CallResult<T> = Result<T, CallError>

interface Answer<T> {
  data?: T
  error?: unknown
  response: Response
}

async function call<T>(send: () => Promise<Answer<T>>): Promise<CallResult<T>> {
  let answer: Answer<T>
  try {
    answer = await send()
  } catch {
    return fail({ kind: 'offline' })
  }
  const { data, error, response } = answer
  if (response.ok) return ok(data as T)
  // Middleware errors may still be plain text; then only the status is known.
  const code = typeof error === 'object' && error !== null && 'code' in error ? (error.code as ProblemCode) : undefined
  const retryAfter = Number(response.headers.get('Retry-After') ?? NaN)
  return fail({ kind: 'problem', status: response.status, code, retryAfter: Number.isFinite(retryAfter) ? retryAfter : undefined })
}

export function requestLoginCode(client: ApiClient, email: string): Promise<CallResult<void>> {
  return call(() => client.POST('/auth/login-code', { body: { email } }))
}

export function verifyLoginCode(client: ApiClient, email: string, code: string): Promise<CallResult<LoginResult>> {
  return call(() => client.POST('/auth/login-code/verify', { body: { email, code } }))
}

export function logout(client: ApiClient): Promise<CallResult<void>> {
  return call(() => client.POST('/auth/logout'))
}

export function getMe(client: ApiClient): Promise<CallResult<User>> {
  return call(() => client.GET('/me'))
}

export function updateMe(client: ApiClient, update: UserUpdate): Promise<CallResult<User>> {
  return call(() => client.PATCH('/me', { body: update }))
}

export function deleteMe(client: ApiClient): Promise<CallResult<void>> {
  return call(() => client.DELETE('/me'))
}

export function listSessions(client: ApiClient): Promise<CallResult<Session[]>> {
  return call(() => client.GET('/auth/sessions'))
}

export function revokeSession(client: ApiClient, sessionId: string): Promise<CallResult<void>> {
  return call(() => client.DELETE('/auth/sessions/{sessionId}', { params: { path: { sessionId } } }))
}

// Google is a navigation, not a fetch: the server answers with redirects and
// comes back to `returnTo`, or to /login?error=google if it failed. The prefix
// is the client's baseUrl (web/src/api/client.ts).
export function googleSignInUrl(returnTo?: string): string {
  const query = returnTo ? `?${new URLSearchParams({ returnTo }).toString()}` : ''
  return `/api/v1/auth/google${query}`
}

// True when the page is the server's redirect after a failed Google sign-in.
export function isGoogleFailure(url: URL): boolean {
  return url.pathname === '/login' && url.searchParams.get('error') === 'google'
}
