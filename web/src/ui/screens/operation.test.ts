import { describe, expect, it } from 'vitest'
import { problemText } from './operation'

const t = ((key: string) => key) as Parameters<typeof problemText>[0]

describe('the text of a refused operation', () => {
  it('explains that the network is needed', () => {
    expect(problemText(t, { kind: 'offline' })).toBe('common.onlineOnly')
  })

  it('prefers what the screen says about that code', () => {
    const error = { kind: 'problem', status: 409, code: 'nonzero_balance' } as const
    expect(problemText(t, error, { nonzero_balance: 'Queda a mano primero.' })).toBe('Queda a mano primero.')
  })

  it('has a text for the rest, unknown codes included', () => {
    expect(problemText(t, { kind: 'problem', status: 400, code: 'validation_failed' })).toBe('problems.invalid')
    expect(problemText(t, { kind: 'problem', status: 429, code: 'rate_limited' })).toBe('problems.rateLimited')
    expect(problemText(t, { kind: 'problem', status: 404, code: 'not_found' })).toBe('problems.gone')
    expect(problemText(t, { kind: 'problem', status: 500 })).toBe('problems.failed')
  })
})
