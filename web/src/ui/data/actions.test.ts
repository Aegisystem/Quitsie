import createClient from 'openapi-fetch'
import { describe, expect, it, vi } from 'vitest'
import type { paths } from '../../api/schema.gen'
import { createActions } from './actions'

function setup(answer: () => Response) {
  const requests: Request[] = []
  const sync = vi.fn(() => Promise.resolve())
  const api = createClient<paths>({
    baseUrl: 'http://quitsie.test/api/v1',
    fetch: (request) => {
      requests.push(request)
      return Promise.resolve(answer())
    },
  })
  return { actions: createActions({ api, engine: { sync } }), requests, sync }
}

const problem = (status: number, code: string) =>
  new Response(JSON.stringify({ title: code, status, code }), { status, headers: { 'Content-Type': 'application/problem+json' } })

const id = '0190a000-0000-7000-8000-000000000001'

describe('deleting and restoring records', () => {
  it.each([
    ['deleteRecord', 'expense', 'DELETE', `/expenses/${id}`],
    ['deleteRecord', 'settlement', 'DELETE', `/settlements/${id}`],
    ['restoreRecord', 'expense', 'POST', `/expenses/${id}/restore`],
    ['restoreRecord', 'settlement', 'POST', `/settlements/${id}/restore`],
  ] as const)('%s of a %s calls the API and then syncs', async (action, kind, method, path) => {
    const { actions, requests, sync } = setup(() => new Response(null, { status: 204 }))
    await expect(actions[action](kind, id)).resolves.toBe('done')
    expect(requests.map(({ method, url }) => [method, new URL(url).pathname])).toEqual([[method, `/api/v1${path}`]])
    expect(sync).toHaveBeenCalledOnce()
  })

  it('reports a former member and leaves the replica as it was', async () => {
    const { actions, sync } = setup(() => problem(409, 'invalid_participants'))
    await expect(actions.deleteRecord('expense', id)).resolves.toBe('formerMember')
    await expect(actions.restoreRecord('settlement', id)).resolves.toBe('formerMember')
    expect(sync).not.toHaveBeenCalled()
  })

  it('reports a Balance that would go past the limit and leaves the replica as it was', async () => {
    const { actions, sync } = setup(() => problem(400, 'invalid_amounts'))
    await expect(actions.deleteRecord('expense', id)).resolves.toBe('balance')
    await expect(actions.restoreRecord('settlement', id)).resolves.toBe('balance')
    expect(sync).not.toHaveBeenCalled()
  })

  it('does not read any other 409 as a former member', async () => {
    const { actions } = setup(() => problem(409, 'version_conflict'))
    await expect(actions.deleteRecord('expense', id)).resolves.toBe('failed')
  })

  it('syncs when the record is gone, so that it leaves the screen', async () => {
    const { actions, sync } = setup(() => problem(404, 'not_found'))
    await expect(actions.deleteRecord('settlement', id)).resolves.toBe('gone')
    expect(sync).toHaveBeenCalledOnce()
  })

  it('says so when the server fails', async () => {
    const { actions, sync } = setup(() => problem(500, 'internal_error'))
    await expect(actions.restoreRecord('expense', id)).resolves.toBe('failed')
    expect(sync).not.toHaveBeenCalled()
  })

  it('says so when no answer comes back', async () => {
    const { actions } = setup(() => {
      throw new TypeError('Failed to fetch')
    })
    await expect(actions.deleteRecord('expense', id)).resolves.toBe('offline')
  })
})
