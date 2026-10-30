import type { NewExpense, NewSettlement, RecordResult } from '../api/client'
import type { QueuedRecord, Replica } from '../local'
import { type Api, SyncFailure } from './api'

// POST /records takes at most this many Expenses and this many Settlements.
const batchSize = 100

// Sends the UnsyncedRecords (POST /records), oldest first. An accepted record
// leaves the queue; a rejected one stays, as a RejectedRecord with its reason.
// A record the response does not mention stays unsynced for the next push.
export async function push(replica: Replica, api: Api): Promise<void> {
  const unsynced = await replica.queue.where({ status: 'unsynced' }).sortBy('id')
  const expenses = unsynced.flatMap((record) => (record.kind === 'expense' ? [record.expense] : []))
  const settlements = unsynced.flatMap((record) => (record.kind === 'settlement' ? [record.settlement] : []))

  while (expenses.length > 0 || settlements.length > 0) {
    const body = { expenses: expenses.splice(0, batchSize), settlements: settlements.splice(0, batchSize) }
    const { data, response } = await api.POST('/records', { body })
    if (!data) throw new SyncFailure(response.status)
    await settle(replica, body, data.results)
  }
}

async function settle(
  replica: Replica,
  sent: { expenses: NewExpense[]; settlements: NewSettlement[] },
  results: RecordResult[],
): Promise<void> {
  const sentIds = new Set([...sent.expenses, ...sent.settlements].map(({ id }) => id))
  await replica.transaction('rw', replica.queue, async () => {
    for (const result of results) {
      const record = await replica.queue.get(result.id)
      if (!sentIds.has(result.id) || record?.status !== 'unsynced') continue
      await replica.queue.put(outcome(record, result))
    }
  })
}

// Accepted records stay in the table, out of the queue, until a pull brings
// them into the replica, so they never vanish from the screens in between.
function outcome(record: QueuedRecord, result: RecordResult): QueuedRecord {
  if (result.status === 'accepted') return { ...record, status: 'accepted' }
  return { ...record, status: 'rejected', reason: result.reason ?? 'invalid_record', ...(result.detail ? { detail: result.detail } : {}) }
}
