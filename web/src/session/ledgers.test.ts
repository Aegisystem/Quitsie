import createClient from 'openapi-fetch'
import { describe, expect, it } from 'vitest'
import type { paths } from '../api/schema.gen'
import { createLedgerCalls } from './ledgers'

const problem = (status: number, code: string, extra: object = {}) =>
  new Response(JSON.stringify({ title: code, status, code, ...extra }), { status, headers: { 'Content-Type': 'application/problem+json' } })
const json = (status: number, body: unknown) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

function setup(answer: (request: Request) => Response) {
  const requests: Request[] = []
  let syncs = 0
  const client = createClient<paths>({
    baseUrl: 'http://quitsie.test/api/v1',
    fetch: async (request: Request) => {
      requests.push(request)
      return answer(request)
    },
  })
  const calls = createLedgerCalls({
    client,
    sync: async () => {
      syncs++
    },
  })
  return { calls, requests, syncs: () => syncs }
}

const group = { id: 'g', name: 'Viaje', currency: 'EUR', simplifyDebts: true, createdAt: '2026-10-31T12:00:00Z', version: 1 }

describe('ledger calls', () => {
  it('syncs before answering, so the next screen finds the change', async () => {
    const { calls, syncs } = setup(() => json(201, group))
    const result = await calls.createGroup({ name: 'Viaje', currency: 'EUR', simplifyDebts: true })
    expect(result).toEqual({ ok: true, value: group })
    expect(syncs()).toBe(1)
  })

  it('syncs after a refusal too: the replica is behind the server', async () => {
    const { calls, syncs } = setup(() => problem(409, 'nonzero_balance'))
    const result = await calls.leaveGroup('g')
    expect(result).toMatchObject({ ok: false, error: { kind: 'problem', status: 409, code: 'nonzero_balance' } })
    expect(syncs()).toBe(1)
  })

  it('does not sync without an answer', async () => {
    const { calls, syncs } = setup(() => {
      throw new TypeError('Failed to fetch')
    })
    expect(await calls.deleteGroup('g')).toEqual({ ok: false, error: { kind: 'offline' } })
    expect(syncs()).toBe(0)
  })

  it('brings the records that would lose decimals', async () => {
    const { calls } = setup(() => problem(409, 'currency_precision_loss', { expenseIds: ['e'], settlementIds: ['s'] }))
    const result = await calls.updateGroup('g', { currency: 'CLP' })
    expect(result).toMatchObject({ ok: false, error: { code: 'currency_precision_loss', lossy: { expenseIds: ['e'], settlementIds: ['s'] } } })
  })

  it('finds the Invite link of each kind of ledger, and showing it changes nothing', async () => {
    const { calls, requests, syncs } = setup(() => json(200, { token: 't' }))
    await calls.inviteLink({ id: 'g', kind: 'group' })
    await calls.inviteLink({ id: 'f', kind: 'friendship' })
    expect(requests.map(({ method, url }) => `${method} ${new URL(url).pathname}`)).toEqual([
      'GET /api/v1/groups/g/invite-link',
      'GET /api/v1/friendships/f/invite-link',
    ])
    expect(syncs()).toBe(0)
  })

  it('readmitting someone removed is a change', async () => {
    const { calls, requests, syncs } = setup(() => new Response(null, { status: 204 }))
    expect(await calls.readmitMember('g', 'm')).toEqual({ ok: true, value: undefined })
    expect(`${requests[0]?.method} ${new URL(requests[0]?.url ?? '').pathname}`).toBe('POST /api/v1/groups/g/members/m/readmit')
    expect(syncs()).toBe(1)
  })

  it('regenerating an Invite link is a change', async () => {
    const { calls, requests, syncs } = setup(() => json(200, { token: 'new' }))
    await calls.regenerateInviteLink({ id: 'f', kind: 'friendship' })
    expect(`${requests[0]?.method} ${new URL(requests[0]?.url ?? '').pathname}`).toBe('POST /api/v1/friendships/f/invite-link')
    expect(syncs()).toBe(1)
  })
})
