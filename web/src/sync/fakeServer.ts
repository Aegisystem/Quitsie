import type { Changes, Group, Member } from '../api/client'

// Test double for the Go server behind `fetch`: it keeps a version counter and
// answers GET /sync like docs/arquitectura.md describes. Only the tests import it.

export const me = '00000000-0000-7000-8000-00000000000a'

export interface FakeGroup {
  id: string
  member: Record<string, string> // memberId by name; the signed-in User is 'Yo'
}

export function fakeServer() {
  let counter = 0
  const groups = new Map<string, Group>()
  const members = new Map<string, Member>()

  const json = (status: number, body: unknown) =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

  function pull(since: number): Changes {
    const newer = <T extends { version: number }>(rows: Map<string, T>) => [...rows.values()].filter((row) => row.version > since)
    return {
      cursor: counter,
      hasMore: false,
      groups: newer(groups),
      friendships: [],
      members: newer(members),
      expenses: [],
      settlements: [],
      activities: [],
      debts: [],
      droppedLedgers: [],
    }
  }

  return {
    async fetch(request: Request): Promise<Response> {
      const url = new URL(request.url)
      if (request.method === 'GET' && url.pathname === '/api/v1/sync') {
        return json(200, pull(Number(url.searchParams.get('since'))))
      }
      return json(404, { title: 'Not found', status: 404, code: 'not_found' })
    },

    addGroup({ name, currency, members: names }: { name: string; currency: string; members: string[] }): FakeGroup {
      const version = ++counter
      const id = crypto.randomUUID()
      groups.set(id, { id, name, currency, simplifyDebts: true, createdAt: new Date().toISOString(), version })
      const member: Record<string, string> = {}
      for (const memberName of ['Yo', ...names]) {
        const memberId = crypto.randomUUID()
        member[memberName] = memberId
        members.set(memberId, {
          id: memberId,
          ledgerId: id,
          name: memberName,
          status: 'active',
          version,
          ...(memberName === 'Yo' ? { userId: me } : {}),
        })
      }
      return { id, member }
    },
  }
}

export type FakeServer = ReturnType<typeof fakeServer>
