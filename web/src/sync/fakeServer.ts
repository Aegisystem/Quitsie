import type { Activity, Changes, Expense, Group, LedgerDebts, Member, Settlement } from '../api/client'

// Test double for the Go server behind `fetch`: it keeps a version counter and
// answers GET /sync like docs/arquitectura.md describes. Only the tests import it.

export const me = '00000000-0000-7000-8000-00000000000a'

export interface FakeGroup {
  id: string
  member: Record<string, string> // memberId by name; the signed-in User is 'Yo'
}

// `versionsPerPull` cuts each GET /sync short, as the real server does with `hasMore`.
export function fakeServer({ versionsPerPull = Infinity }: { versionsPerPull?: number } = {}) {
  let counter = 0
  const groups = new Map<string, Group>()
  const members = new Map<string, Member>()
  const expenses = new Map<string, Expense>()
  const settlements = new Map<string, Settlement>()
  const activities = new Map<string, Activity>()
  const debts = new Map<string, LedgerDebts & { version: number }>()
  const pulls: number[] = []

  const json = (status: number, body: unknown) =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })

  function pull(since: number): Changes {
    const cursor = Math.min(counter, since + versionsPerPull)
    const newer = <T extends { version: number }>(rows: Map<string, T>) =>
      [...rows.values()].filter((row) => row.version > since && row.version <= cursor)
    return {
      cursor,
      hasMore: cursor < counter,
      groups: newer(groups),
      friendships: [],
      members: newer(members),
      expenses: newer(expenses),
      settlements: newer(settlements),
      activities: newer(activities),
      debts: newer(debts).map(({ ledgerId, debts }) => ({ ledgerId, debts })),
      droppedLedgers: [],
    }
  }

  function log(ledgerId: string, kind: Activity['kind'], version: number) {
    const id = crypto.randomUUID()
    activities.set(id, { id, ledgerId, userId: me, kind, details: {}, createdAt: new Date().toISOString(), version })
  }

  // Amounts by Member name, so the tests read like the screens.
  const shares = (group: FakeGroup, amounts: Record<string, number>) =>
    Object.entries(amounts).map(([name, amount]) => ({ memberId: group.member[name] ?? name, amount }))

  return {
    // The `since` of every GET /sync, in order.
    pulls,
    version: () => counter,

    async fetch(request: Request): Promise<Response> {
      const url = new URL(request.url)
      if (request.method === 'GET' && url.pathname === '/api/v1/sync') {
        const since = Number(url.searchParams.get('since'))
        pulls.push(since)
        return json(200, pull(since))
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
      log(id, 'group_created', version)
      return { id, member }
    },

    addExpense(
      group: FakeGroup,
      { description, paid, owed }: { description: string; paid: Record<string, number>; owed: Record<string, number> },
    ): string {
      const id = crypto.randomUUID()
      const ledger = groups.get(group.id)!
      const total = Object.values(paid).reduce((sum, amount) => sum + amount, 0)
      const now = new Date().toISOString()
      expenses.set(id, {
        id,
        ledgerId: group.id,
        description,
        date: now.slice(0, 10),
        currency: ledger.currency,
        total,
        splitMethod: 'exact',
        paidShares: shares(group, paid),
        owedShares: shares(group, owed),
        createdBy: me,
        createdAt: now,
        version: ++counter,
      })
      log(group.id, 'expense_created', counter)
      return id
    },

    addSettlement(group: FakeGroup, { from, to, amount }: { from: string; to: string; amount: number }): string {
      const id = crypto.randomUUID()
      const now = new Date().toISOString()
      settlements.set(id, {
        id,
        ledgerId: group.id,
        fromMemberId: group.member[from]!,
        toMemberId: group.member[to]!,
        amount,
        currency: groups.get(group.id)!.currency,
        date: now.slice(0, 10),
        createdBy: me,
        createdAt: now,
        version: ++counter,
      })
      log(group.id, 'settlement_created', counter)
      return id
    },

    setDebts(group: FakeGroup, list: { from: string; to: string; amount: number }[]) {
      const { currency } = groups.get(group.id)!
      debts.set(group.id, {
        ledgerId: group.id,
        debts: list.map(({ from, to, amount }) => ({
          fromMemberId: group.member[from]!,
          toMemberId: group.member[to]!,
          amount,
          currency,
        })),
        version: ++counter,
      })
    },

    deleteExpense(id: string) {
      const expense = expenses.get(id)!
      expenses.set(id, { ...expense, deletedAt: new Date().toISOString(), version: ++counter })
      log(expense.ledgerId, 'expense_deleted', counter)
    },
  }
}

export type FakeServer = ReturnType<typeof fakeServer>
