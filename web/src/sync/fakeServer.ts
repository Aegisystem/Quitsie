import type { Activity, DroppedLedger, Expense, Friendship, Group, LedgerDebts, Member, Problem, RecordBatch, RecordResult, Settlement } from '../api/client'
import type { PulledChanges } from './pull'

// Test double for the Go server behind `fetch`: it keeps a version counter and
// answers GET /sync like docs/arquitectura.md describes. Only the tests import it.

export const me = '00000000-0000-7000-8000-00000000000a'

export interface FakeGroup {
  id: string
  member: Record<string, string> // memberId by name; the signed-in User is 'Yo'
}

// Everything the server stores, in one value so a backup is a copy of it.
function emptyDatabase() {
  return {
    counter: 0,
    groups: new Map<string, Group>(),
    friendships: new Map<string, Friendship>(),
    members: new Map<string, Member>(),
    expenses: new Map<string, Expense>(),
    settlements: new Map<string, Settlement>(),
    activities: new Map<string, Activity>(),
    debts: new Map<string, LedgerDebts & { version: number }>(),
    // Ledgers the User lost, and the version at which it happened.
    dropped: new Map<string, DroppedLedger & { version: number; announce: boolean }>(),
  }
}

export type Backup = ReturnType<typeof emptyDatabase>

// `versionsPerPull` cuts each GET /sync short, as the real server does with `hasMore`.
export function fakeServer({ versionsPerPull = Infinity }: { versionsPerPull?: number } = {}) {
  let db = emptyDatabase()
  const pulls: number[] = []
  const pushes: RecordBatch[] = []
  let reachable = true
  let failure: { status: 401 | 503; method?: 'GET' | 'POST' } | undefined
  let afterNextPush: 'drop the connection' | 'lose the answer' | undefined
  let active = 0
  let busiest = 0

  const json = (status: number, body: unknown) =>
    new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
  const problem = (status: number, code: Problem['code']) =>
    new Response(JSON.stringify({ title: code, status, code } satisfies Problem), {
      status,
      headers: { 'Content-Type': 'application/problem+json' },
    })

  const visible = (ledgerId: string) => (db.groups.has(ledgerId) || db.friendships.has(ledgerId)) && !db.dropped.has(ledgerId)

  function pull(since: number): PulledChanges {
    const cursor = Math.min(db.counter, since + versionsPerPull)
    const inPage = ({ version }: { version: number }) => version > since && version <= cursor
    const newer = <T extends { version: number; ledgerId?: string; id?: string }>(rows: Map<string, T>) =>
      [...rows.values()].filter((row) => inPage(row) && visible(row.ledgerId ?? row.id!))
    return {
      cursor,
      hasMore: cursor < db.counter,
      groups: newer(db.groups),
      friendships: newer(db.friendships),
      members: newer(db.members),
      expenses: newer(db.expenses),
      settlements: newer(db.settlements),
      activities: newer(db.activities),
      debts: newer(db.debts).map(({ ledgerId, debts }) => ({ ledgerId, debts })),
      droppedLedgers: [...db.dropped.values()]
        .filter((drop) => drop.announce && inPage(drop))
        .map(({ ledgerId, reason }) => ({ ledgerId, reason })),
      visibleLedgerIds: [...db.groups.keys(), ...db.friendships.keys()].filter(visible),
    }
  }

  function log(ledgerId: string, kind: Activity['kind'], version: number) {
    const id = crypto.randomUUID()
    db.activities.set(id, { id, ledgerId, userId: me, kind, details: {}, createdAt: new Date().toISOString(), version })
  }

  function rejection({ ledgerId, currency }: { ledgerId: string; currency: string }): RecordResult['reason'] {
    if (!visible(ledgerId)) return 'ledger_unavailable'
    const group = db.groups.get(ledgerId)
    if (group && group.currency !== currency) return 'currency_mismatch' // in a Friendship each Expense has its own
    return undefined
  }

  // POST /records: each record is accepted or rejected on its own, and a record
  // the server already has is accepted again without changes.
  function push({ expenses, settlements }: RecordBatch): RecordResult[] {
    const results: RecordResult[] = []
    for (const expense of expenses) {
      const known = db.expenses.get(expense.id)
      if (known) {
        results.push({ id: expense.id, kind: 'expense', status: 'accepted', version: known.version })
        continue
      }
      const reason = rejection(expense)
      if (reason) {
        results.push({ id: expense.id, kind: 'expense', status: 'rejected', reason })
        continue
      }
      const version = ++db.counter
      db.expenses.set(expense.id, { ...expense, createdBy: me, createdAt: new Date().toISOString(), version })
      log(expense.ledgerId, 'expense_created', version)
      results.push({ id: expense.id, kind: 'expense', status: 'accepted', version })
    }
    for (const settlement of settlements) {
      const known = db.settlements.get(settlement.id)
      if (known) {
        results.push({ id: settlement.id, kind: 'settlement', status: 'accepted', version: known.version })
        continue
      }
      const reason = rejection(settlement)
      if (reason) {
        results.push({ id: settlement.id, kind: 'settlement', status: 'rejected', reason })
        continue
      }
      const version = ++db.counter
      db.settlements.set(settlement.id, { ...settlement, createdBy: me, createdAt: new Date().toISOString(), version })
      log(settlement.ledgerId, 'settlement_created', version)
      results.push({ id: settlement.id, kind: 'settlement', status: 'accepted', version })
    }
    return results
  }

  function addMembers(ledgerId: string, names: string[], version: number): Record<string, string> {
    const member: Record<string, string> = {}
    for (const name of ['Yo', ...names]) {
      const id = crypto.randomUUID()
      member[name] = id
      db.members.set(id, { id, ledgerId, name, status: 'active', version, ...(name === 'Yo' ? { userId: me } : {}) })
    }
    return member
  }

  // Amounts by Member name, so the tests read like the screens.
  const shares = (group: FakeGroup, amounts: Record<string, number>) =>
    Object.entries(amounts).map(([name, amount]) => ({ memberId: group.member[name] ?? name, amount }))

  async function answer(request: Request): Promise<Response> {
    if (!reachable) throw new TypeError('Failed to fetch')
    if (failure && (!failure.method || failure.method === request.method)) {
      if (failure.status === 401) return problem(401, 'unauthenticated')
      return new Response('Service Unavailable', { status: failure.status })
    }
    const url = new URL(request.url)
    if (request.method === 'GET' && url.pathname === '/api/v1/sync') {
      const since = Number(url.searchParams.get('since'))
      pulls.push(since)
      if (since > db.counter) return problem(410, 'cursor_invalid')
      return json(200, pull(since))
    }
    if (request.method === 'POST' && url.pathname === '/api/v1/records') {
      const batch = (await request.json()) as RecordBatch
      pushes.push(batch)
      if (batch.expenses.length > 100 || batch.settlements.length > 100) return problem(400, 'validation_failed')
      // The real server decodes the whole body first: one bad date fails the batch.
      const records = [...batch.expenses, ...batch.settlements]
      if (records.some(({ date }) => !/^\d{4}-\d{2}-\d{2}$/.test(date))) return problem(400, 'validation_failed')
      const results = push(batch)
      if (afterNextPush) {
        reachable = afterNextPush === 'lose the answer'
        afterNextPush = undefined
        throw new TypeError('Failed to fetch')
      }
      return json(200, { results })
    }
    return problem(404, 'not_found')
  }

  return {
    // The `since` of every GET /sync, and the batch of every POST /records, in order.
    pulls,
    pushes,
    version: () => db.counter,

    // The network fails, as fetch does when there is no route to the server.
    setReachable(value: boolean) {
      reachable = value
    },
    // Requests answer with this status, or normally again with undefined.
    failWith(status: 401 | 503 | undefined, method?: 'GET' | 'POST') {
      failure = status && { status, method }
    },
    // The server stores the next push, but the device never hears back.
    loseNextPushAnswer() {
      afterNextPush = 'lose the answer'
    },
    // The same, and the network stays down until setReachable(true).
    dropConnectionAfterNextPush() {
      afterNextPush = 'drop the connection'
    },
    expensesIn: (group: FakeGroup) => [...db.expenses.values()].filter(({ ledgerId }) => ledgerId === group.id),

    // The most requests that were ever in flight at once.
    busiest: () => busiest,

    async fetch(request: Request): Promise<Response> {
      active += 1
      busiest = Math.max(busiest, active)
      await new Promise((resolve) => setTimeout(resolve))
      try {
        return await answer(request)
      } finally {
        active -= 1
      }
    },

    backup: (): Backup => structuredClone(db),
    // Like `litestream restore`: everything after the backup is lost, the counter included.
    restore(backup: Backup) {
      db = structuredClone(backup)
    },

    addGroup({ name, currency, members: names }: { name: string; currency: string; members: string[] }): FakeGroup {
      const version = ++db.counter
      const id = crypto.randomUUID()
      db.groups.set(id, { id, name, currency, simplifyDebts: true, createdAt: new Date().toISOString(), version })
      log(id, 'group_created', version)
      return { id, member: addMembers(id, names, version) }
    },

    addFriendship(friend: string): FakeGroup {
      const version = ++db.counter
      const id = crypto.randomUUID()
      db.friendships.set(id, { id, createdAt: new Date().toISOString(), version })
      log(id, 'friendship_created', version)
      return { id, member: addMembers(id, [friend], version) }
    },

    addExpense(
      group: FakeGroup,
      {
        description,
        paid,
        owed,
        date,
      }: { description: string; paid: Record<string, number>; owed: Record<string, number>; date?: string },
    ): string {
      const id = crypto.randomUUID()
      const now = new Date().toISOString()
      db.expenses.set(id, {
        id,
        ledgerId: group.id,
        description,
        date: date ?? now.slice(0, 10),
        currency: db.groups.get(group.id)!.currency,
        total: Object.values(paid).reduce((sum, amount) => sum + amount, 0),
        splitMethod: 'exact',
        paidShares: shares(group, paid),
        owedShares: shares(group, owed),
        createdBy: me,
        createdAt: now,
        version: ++db.counter,
      })
      log(group.id, 'expense_created', db.counter)
      return id
    },

    addSettlement(group: FakeGroup, { from, to, amount, date }: { from: string; to: string; amount: number; date?: string }): string {
      const id = crypto.randomUUID()
      const now = new Date().toISOString()
      db.settlements.set(id, {
        id,
        ledgerId: group.id,
        fromMemberId: group.member[from]!,
        toMemberId: group.member[to]!,
        amount,
        currency: db.groups.get(group.id)!.currency,
        date: date ?? now.slice(0, 10),
        createdBy: me,
        createdAt: now,
        version: ++db.counter,
      })
      log(group.id, 'settlement_created', db.counter)
      return id
    },

    setDebts(group: FakeGroup, list: { from: string; to: string; amount: number }[]) {
      const { currency } = db.groups.get(group.id)!
      db.debts.set(group.id, {
        ledgerId: group.id,
        debts: list.map(({ from, to, amount }) => ({
          fromMemberId: group.member[from]!,
          toMemberId: group.member[to]!,
          amount,
          currency,
        })),
        version: ++db.counter,
      })
    },

    // Relabels the Group's amounts; queued records in the old currency get rejected.
    setCurrency(group: FakeGroup, currency: string) {
      const ledger = db.groups.get(group.id)!
      db.groups.set(group.id, { ...ledger, currency, version: ++db.counter })
      log(group.id, 'group_currency_changed', db.counter)
    },

    // The User leaves the Group or loses it. The pull says so in droppedLedgers
    // unless `announce` is false, and always leaves it out of visibleLedgerIds.
    removeMe(group: FakeGroup, { reason, announce = true }: { reason: DroppedLedger['reason']; announce?: boolean }) {
      db.dropped.set(group.id, { ledgerId: group.id, reason, version: ++db.counter, announce })
    },

    deleteExpense(id: string) {
      const expense = db.expenses.get(id)!
      db.expenses.set(id, { ...expense, deletedAt: new Date().toISOString(), version: ++db.counter })
      log(expense.ledgerId, 'expense_deleted', db.counter)
    },
  }
}

export type FakeServer = ReturnType<typeof fakeServer>
