import { liveQuery, type Observable } from 'dexie'
import type { CategorizedExpense, CurrencyCode } from '../../domain'
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

// The Expenses the User gave a Category, newest first, for suggestCategory.
export function useCategoryHistory(userId: string): CategorizedExpense[] {
  const [history = []] = useLive(`categoryHistory:${userId}`, () => liveCategoryHistory(replica, userId))
  return history
}

// What suggestCategory learns from: the User's own Expenses with a Category,
// those still in the queue first (the newest), then the synced ones from the
// newest. Rejected and deleted Expenses do not count, and only the last 500 do.
export function liveCategoryHistory(replica: Replica, userId: string): Observable<CategorizedExpense[]> {
  return liveQuery(async () => {
    const [expenses, queue] = await Promise.all([replica.expenses.toArray(), replica.queue.toArray()])
    const queued = queue
      .flatMap((record) => (record.kind === 'expense' && record.status !== 'rejected' ? [record.expense] : []))
      .reverse()
    const synced = expenses
      .filter(({ createdBy, deletedAt }) => createdBy === userId && !deletedAt)
      .sort((a, b) => b.createdAt.localeCompare(a.createdAt))
    return [...queued, ...synced]
      .flatMap(({ description, category }) => (category ? [{ description, category }] : []))
      .slice(0, 500)
  })
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
