import type { Changes } from '../api/client'
import type { Replica } from '../local'
import { type Api, SyncFailure } from './api'

// TODO(contract): another session is adding `visibleLedgerIds` to Changes in
// openapi.yaml. Until `npm run generate` brings it, it is declared here with the
// same name, optional so a server without it still works; delete this then.
export type PulledChanges = Changes & {
  // Every ledger the User can see. The replica drops any other.
  visibleLedgerIds?: string[]
}

// Brings everything that changed since the replica's cursor (GET /sync), page
// by page while the server says it has more.
export async function pull(replica: Replica, api: Api): Promise<void> {
  for (;;) {
    const since = (await replica.meta.get('cursor'))?.value ?? 0
    const { data, response } = await api.GET('/sync', { params: { query: { since } } })
    // 410: the cursor is ahead of the server, which restored an older backup.
    // The replica may hold rows the server lost, so it starts over from 0.
    if (response.status === 410 && since > 0) {
      await clearReplica(replica)
      continue
    }
    if (!data) throw new SyncFailure(response.status)
    await applyChanges(replica, data)
    if (!data.hasMore) return
  }
}

async function clearReplica(replica: Replica): Promise<void> {
  const tables = [replica.ledgers, replica.members, replica.expenses, replica.settlements, replica.activities, replica.debts, replica.meta]
  await replica.transaction('rw', tables, () => Promise.all(tables.map((table) => table.clear())))
}

async function applyChanges(replica: Replica, changes: PulledChanges): Promise<void> {
  const tables = [replica.ledgers, replica.members, replica.expenses, replica.settlements, replica.activities, replica.debts, replica.meta]
  await replica.transaction('rw', [...tables, replica.queue], async () => {
    await replica.ledgers.bulkPut([
      ...changes.groups.map((group) => ({ ...group, kind: 'group' as const })),
      ...changes.friendships.map((friendship) => ({ ...friendship, kind: 'friendship' as const })),
    ])
    await replica.members.bulkPut(changes.members)
    await replica.expenses.bulkPut(changes.expenses)
    await replica.settlements.bulkPut(changes.settlements)
    await replica.activities.bulkPut(changes.activities)
    await replica.debts.bulkPut(changes.debts)
    for (const { ledgerId } of changes.droppedLedgers) await dropLedger(replica, ledgerId)
    if (changes.visibleLedgerIds) {
      const visible = new Set(changes.visibleLedgerIds)
      for (const ledgerId of await replica.ledgers.toCollection().primaryKeys()) {
        if (!visible.has(ledgerId)) await dropLedger(replica, ledgerId)
      }
    }
    await leaveQueueOnceInReplica(replica)
    await replica.meta.put({ key: 'cursor', value: changes.cursor })
  })
}

// A queued record whose row is now in the replica is on the server: it leaves
// the table, whether it was accepted or its acceptance never reached the device.
// Rejected records only leave when the User fixes or discards them.
async function leaveQueueOnceInReplica(replica: Replica): Promise<void> {
  for (const record of await replica.queue.where('status').anyOf('unsynced', 'accepted').toArray()) {
    const row = await (record.kind === 'expense' ? replica.expenses : replica.settlements).get(record.id)
    if (row?.ledgerId === record.ledgerId) await replica.queue.delete(record.id)
  }
}

// Deletes a ledger the User can no longer see, with everything in it.
async function dropLedger(replica: Replica, ledgerId: string): Promise<void> {
  await replica.ledgers.delete(ledgerId)
  await replica.debts.delete(ledgerId)
  for (const table of [replica.members, replica.expenses, replica.settlements, replica.activities]) {
    await table.where({ ledgerId }).delete()
  }
}
