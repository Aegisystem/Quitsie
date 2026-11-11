import { describe, expect, it } from 'vitest'
import { signInProblemText, waitToAskAgain } from './signInProblem'

const t = ((key: string, options?: { count?: number }) =>
  options?.count === undefined ? key : `${key}:${options.count}`) as Parameters<typeof signInProblemText>[0]

describe('what the sign-in screen says when a request fails', () => {
  it('tells apart the server running out of emails from asking too often', () => {
    const spent = { kind: 'problem', status: 503, code: 'mail_budget_spent' } as const
    expect(signInProblemText(t, { ...spent, retryAfter: 2700 })).toBe('signIn.mailBudgetSpentFor:45')
    expect(signInProblemText(t, { ...spent, retryAfter: 5 * 3600 })).toBe('signIn.mailBudgetSpentForHours:5')
    expect(signInProblemText(t, spent)).toBe('signIn.mailBudgetSpent')

    const limited = { kind: 'problem', status: 429, code: 'rate_limited' } as const
    expect(signInProblemText(t, { ...limited, retryAfter: 2700 })).toBe('signIn.rateLimitedFor:45')
    expect(signInProblemText(t, { ...limited, retryAfter: 86400 })).toBe('signIn.rateLimitedForHours:24')
    expect(signInProblemText(t, { ...limited, retryAfter: 40 })).toBe('signIn.rateLimitedForSeconds:40')
  })

  // The server sent nothing, but a code this browser asked for before still
  // works, so the screen goes on to it.
  it('knows when an earlier code may still be typed, and how long until another', () => {
    const limited = { kind: 'problem', status: 429, code: 'rate_limited' } as const
    expect(waitToAskAgain({ ...limited, retryAfter: 40 })).toBe(40)
    expect(waitToAskAgain({ ...limited, retryAfter: 3600 })).toBe(3600)
    // An email locked for wrong guesses takes no code until then.
    expect(waitToAskAgain({ ...limited, retryAfter: 86400 })).toBeUndefined()
    expect(waitToAskAgain(limited)).toBeUndefined()
    expect(waitToAskAgain({ kind: 'problem', status: 503, code: 'mail_budget_spent', retryAfter: 40 })).toBeUndefined()
    expect(waitToAskAgain({ kind: 'problem', status: 400, code: 'validation_failed' })).toBeUndefined()
    expect(waitToAskAgain({ kind: 'offline' })).toBeUndefined()
  })

  it('has a text for the rest', () => {
    expect(signInProblemText(t, { kind: 'offline' })).toBe('signIn.offline')
    expect(signInProblemText(t, { kind: 'problem', status: 400, code: 'validation_failed' })).toBe('signIn.invalidEmail')
    expect(signInProblemText(t, { kind: 'problem', status: 500 })).toBe('signIn.failed')
  })
})
