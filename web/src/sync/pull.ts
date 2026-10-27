import type { Changes } from '../api/client'
import type { Replica } from '../local'
import type { Api } from './engine'

// Brings everything that changed since the replica's cursor (GET /sync), page
// by page while the server says it has more.
export async function pull(replica: Replica, api: Api): Promise<void> {
  for (;;) {
    const since = (await replica.meta.get('cursor'))?.value ?? 0
    const { data, error } = await api.GET('/sync', { params: { query: { since } } })
    if (error) throw new Error(`pull failed: ${error.code}`)
    await applyChanges(replica, data)
    if (!data.hasMore) return
  }
}

async function applyChanges(replica: Replica, changes: Changes): Promise<void> {
  const tables = [replica.ledgers, replica.members, replica.expenses, replica.settlements, replica.activities, replica.debts, replica.meta]
  await replica.transaction('rw', tables, async () => {
    await replica.ledgers.bulkPut([
      ...changes.groups.map((group) => ({ ...group, kind: 'group' as const })),
      ...changes.friendships.map((friendship) => ({ ...friendship, kind: 'friendship' as const })),
    ])
    await replica.members.bulkPut(changes.members)
    await replica.expenses.bulkPut(changes.expenses)
    await replica.settlements.bulkPut(changes.settlements)
    await replica.activities.bulkPut(changes.activities)
    await replica.debts.bulkPut(changes.debts)
    await replica.meta.put({ key: 'cursor', value: changes.cursor })
  })
}
