import { describe, expect, it } from 'vitest'
import { fail, ok } from '../../domain/result'
import { loginMethodsStaleTime, offersGoogle } from './loginMethods'

describe('whether the sign-in screen offers Google', () => {
  it('offers it only when the server says it can sign in with Google', () => {
    expect(offersGoogle(ok({ google: true }))).toBe(true)
    expect(offersGoogle(ok({ google: false }))).toBe(false)
  })

  it('leaves the code alone while the server has not answered or cannot', () => {
    expect(offersGoogle(undefined)).toBe(false)
    expect(offersGoogle(fail({ kind: 'offline' }))).toBe(false)
    expect(offersGoogle(fail({ kind: 'problem', status: 500, code: 'internal_error' }))).toBe(false)
    expect(offersGoogle(fail({ kind: 'problem', status: 429, code: 'rate_limited', retryAfter: 30 }))).toBe(false)
  })

  it('keeps an answer for good and asks again after a failure', () => {
    expect(loginMethodsStaleTime(ok({ google: false }))).toBe(Infinity)
    expect(loginMethodsStaleTime(fail({ kind: 'offline' }))).toBe(0)
    expect(loginMethodsStaleTime(undefined)).toBe(0)
  })
})
