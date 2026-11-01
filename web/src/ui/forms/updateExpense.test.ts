import createClient from 'openapi-fetch'
import { describe, expect, it } from 'vitest'
import type { paths } from '../../api/schema.gen'
import { afterEdit, type ExpenseUpdate, updateExpense } from './updateExpense'

const update: ExpenseUpdate = {
  expectedVersion: 7,
  description: 'Cena',
  date: '2026-10-30',
  currency: 'USD',
  total: 900,
  splitMethod: 'equal',
  paidShares: [{ memberId: 'ana', amount: 900 }],
  owedShares: [{ memberId: 'ana', amount: 900 }],
}

const client = (answer: () => Promise<Response>) => createClient<paths>({ baseUrl: 'http://quitsie.test/api/v1', fetch: answer })

const problem = (status: number, code?: string) => async () =>
  new Response(JSON.stringify({ status, title: 'No', code }), { status, headers: { 'Content-Type': 'application/problem+json' } })

describe('updateExpense', () => {
  it('sends the expected version with a PATCH to the Expense', async () => {
    let sent: Request | undefined
    const fetch = async (request: Request) => {
      sent = request
      return new Response('{}', { status: 200, headers: { 'Content-Type': 'application/json' } })
    }
    const outcome = await updateExpense(createClient<paths>({ baseUrl: 'http://quitsie.test/api/v1', fetch }), 'e1', update)
    expect(outcome).toBe('saved')
    expect(sent?.method).toBe('PATCH')
    expect(sent?.url).toBe('http://quitsie.test/api/v1/expenses/e1')
    expect(await sent?.json()).toMatchObject({ expectedVersion: 7 })
  })

  it.each([
    [409, 'version_conflict', 'conflict'],
    [409, 'currency_mismatch', 'currency'],
    [409, 'invalid_participants', 'participants'],
    [404, 'not_found', 'gone'],
    [400, 'invalid_amounts', 'failed'],
    [500, 'internal_error', 'failed'],
  ])('reads a %i %s as %s', async (status, code, outcome) => {
    expect(await updateExpense(client(problem(status, code)), 'e1', update)).toBe(outcome)
  })

  it('reads a plain text error as a failure', async () => {
    const plain = async () => new Response('Bad Gateway', { status: 502 })
    expect(await updateExpense(client(plain), 'e1', update)).toBe('failed')
  })

  it('says offline when no answer comes back', async () => {
    const unreachable = async () => {
      throw new TypeError('Failed to fetch')
    }
    expect(await updateExpense(client(unreachable), 'e1', update)).toBe('offline')
  })
})

describe('a failed save, once the User edits the form', () => {
  it.each(['currency', 'participants', 'gone', 'offline', 'failed'] as const)('stops showing %s', (outcome) => {
    expect(afterEdit(outcome)).toBeUndefined()
  })

  it('keeps a conflict, which only the new version solves', () => {
    expect(afterEdit('conflict')).toBe('conflict')
  })
})
