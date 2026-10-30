import type { RecordBatch, RecordResult } from '../api/client'
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
    await send(replica, api, { expenses: expenses.splice(0, batchSize), settlements: settlements.splice(0, batchSize) })
  }
}

// A 400 is a batch the server could not even read, which one record with a bad
// date is enough for. Retrying would get the same answer forever with the whole
// queue waiting, so each record goes alone and the one at fault gets rejected.
async function send(replica: Replica, api: Api, batch: RecordBatch): Promise<void> {
  const { data, response } = await api.POST('/records', { body: batch })
  if (data) return settle(replica, data.results)
  if (response.status !== 400) throw new SyncFailure(response.status)

  const alone = [
    ...batch.expenses.map((expense) => ({ expenses: [expense], settlements: [] })),
    ...batch.settlements.map((settlement) => ({ expenses: [], settlements: [settlement] })),
  ]
  if (alone.length === 1) {
    const { id } = batch.expenses[0] ?? batch.settlements[0]!
    return settle(replica, [{ id, status: 'rejected', reason: 'invalid_record' }])
  }
  for (const single of alone) await send(replica, api, single)
}

type Outcome = Pick<RecordResult, 'id' | 'status' | 'reason' | 'detail'>

async function settle(replica: Replica, results: Outcome[]): Promise<void> {
  await replica.transaction('rw', replica.queue, async () => {
    for (const result of results) {
      const record = await replica.queue.get(result.id)
      if (record?.status === 'unsynced') await replica.queue.put(outcome(record, result))
    }
  })
}

function outcome(record: QueuedRecord, { status, reason, detail }: Outcome): QueuedRecord {
  if (status === 'accepted') return { ...record, status }
  return { ...record, status, reason: reason ?? 'invalid_record', ...(detail ? { detail } : {}) }
}
