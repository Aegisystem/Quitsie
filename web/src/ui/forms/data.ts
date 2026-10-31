import { useEffect, useState } from 'react'
import { liveQuery, type Observable } from 'dexie'
import { api } from '../../api/client'
import type { CurrencyCode } from '../../domain'
import { openReplica, type Replica } from '../../local'
import { useSession } from '../../session'
import { createSyncEngine } from '../../sync'

// The app's one replica and sync engine. Who starts the engine is App's business;
// the forms only read and queue.
export const replica = openReplica()
export const engine = createSyncEngine({ replica, api })

// Renders every value of a live query and unsubscribes when the screen goes
// away. `key` names the query. `loaded` tells a query that has not answered yet
// from one that answered undefined, as liveLedger does for a ledger the User
// cannot see.
export function useLive<T>(key: string, query: () => Observable<T>): [value: T | undefined, loaded: boolean] {
  const [latest, setLatest] = useState<{ key: string; value: T }>()
  useEffect(() => {
    const subscription = query().subscribe((value) => setLatest({ key, value }))
    return () => subscription.unsubscribe()
    // The key stands for the query, which is a new function on every render.
  }, [key])
  return latest?.key === key ? [latest.value, true] : [undefined, false]
}

// The signed-in User. App only opens the forms for one, so it is missing only
// for the moment it takes App to react to a sign-out.
export function useUser() {
  const state = useSession()
  return state.status === 'signedIn' ? state.user : undefined
}

// One entry per Expense the User created, in its currency, to tell the currencies
// they use the most. Rejected and deleted Expenses do not count.
export function liveUsedCurrencies(replica: Replica, userId: string): Observable<CurrencyCode[]> {
  return liveQuery(async () => {
    const [expenses, queue] = await Promise.all([replica.expenses.toArray(), replica.queue.toArray()])
    const synced = expenses.filter(({ createdBy, deletedAt }) => createdBy === userId && !deletedAt)
    const queued = queue.flatMap((record) => (record.kind === 'expense' && record.status !== 'rejected' ? [record.expense] : []))
    return [...synced, ...queued].map(({ currency }) => currency)
  })
}
