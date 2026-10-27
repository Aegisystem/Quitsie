import type { Changes } from '../api/client'
import type { Replica } from '../local'
import type { Api } from './engine'

// Brings everything that changed since the replica's cursor (GET /sync).
export async function pull(replica: Replica, api: Api): Promise<void> {
  const since = (await replica.meta.get('cursor'))?.value ?? 0
  const { data, error } = await api.GET('/sync', { params: { query: { since } } })
  if (error) throw new Error(`pull failed: ${error.code}`)
  await applyChanges(replica, data)
}

async function applyChanges(replica: Replica, changes: Changes): Promise<void> {
  await replica.transaction('rw', [replica.ledgers, replica.members, replica.meta], async () => {
    await replica.ledgers.bulkPut([
      ...changes.groups.map((group) => ({ ...group, kind: 'group' as const })),
      ...changes.friendships.map((friendship) => ({ ...friendship, kind: 'friendship' as const })),
    ])
    await replica.members.bulkPut(changes.members)
    await replica.meta.put({ key: 'cursor', value: changes.cursor })
  })
}
