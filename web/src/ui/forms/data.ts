import { liveQuery, type Observable } from 'dexie'
import type { CurrencyCode } from '../../domain'
import type { Replica } from '../../local'
import { replica, useSession } from '../../session'
import { useLive } from '../data'

// The signed-in User. App only opens the forms for one, so it is missing only
// for the moment it takes App to react to a sign-out.
export function useUser() {
  const state = useSession()
  return state.status === 'signedIn' ? state.user : undefined
}

// The currencies the User uses the most, for the currency selectors.
export function useUsedCurrencies(userId: string): CurrencyCode[] {
  const [used = []] = useLive(`usedCurrencies:${userId}`, () => liveUsedCurrencies(replica, userId))
  return used
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
