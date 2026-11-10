import type { Member, NewExpense, OwedShare, SplitMethod } from '../../api/client'
import type { Records, OwedShare as SplitShare } from '../../domain'
import { expenseRecord } from '../../local'
import { formatAmountInput, formatPercentInput } from '../money'
import type { SplitParticipant } from '../split'
import { splitMethods } from '../types'

// What the form starts from when it is not blank: a RejectedRecord to fix or an
// Expense to edit.
export type ExpenseDraft = Omit<NewExpense, 'id' | 'ledgerId'>

// What the Usuario typed for each Member, kept per method so that going through
// the methods loses nothing.
export type SplitValues = Record<SplitMethod, Record<string, string>>

// A method the draft did not use starts empty, and switchMethod fills it from the
// split on screen when the Usuario picks it.
export function initialSplitValues(draft?: ExpenseDraft): SplitValues {
  const values: SplitValues = { equal: {}, exact: {}, percentage: {}, weights: {}, adjustment: {} }
  if (!draft) return values
  const { currency, splitMethod, owedShares } = draft
  for (const { memberId, amount, splitValue } of owedShares) {
    // Any draft knows its amounts, so changing to exact starts from its Cuotas.
    values.exact[memberId] = formatAmountInput(amount, currency)
    if (splitValue === undefined) continue
    if (splitMethod === 'percentage') values.percentage[memberId] = formatPercentInput(splitValue)
    if (splitMethod === 'weights') values.weights[memberId] = String(splitValue)
    if (splitMethod === 'adjustment') values.adjustment[memberId] = formatAmountInput(splitValue, currency)
  }
  return values
}

// Changing method keeps who takes part. A method with nothing typed in it starts
// from the split on screen, so nobody starts from zeros: Montos exactos gets the
// Cuotas, Porcentajes their share of the total, and Proporciones one part each.
// Partes iguales and Ajustes start from who is ticked, which they share.
export function switchMethod(
  to: SplitMethod,
  {
    values,
    included,
    activeIds,
    takingPart,
    shares,
    currency,
  }: {
    values: SplitValues
    included: Record<string, boolean>
    // The active Members, in the order of the list.
    activeIds: readonly string[]
    // The active Members in the split now.
    takingPart: readonly string[]
    // Their Cuotas, when the split closes.
    shares?: ReadonlyMap<string, number>
    currency: string
  },
): { values: SplitValues; included: Record<string, boolean> } {
  const taking = activeIds.filter((id) => takingPart.includes(id))
  if (taking.length === 0) return { values, included }
  const ticked = Object.fromEntries(activeIds.map((id) => [id, taking.includes(id)]))
  const typed = activeIds.some((id) => (values[to][id] ?? '').trim() !== '')
  if (to === 'equal' || to === 'adjustment' || typed) return { values, included: ticked }
  let filled: Record<string, string> | undefined
  switch (to) {
    case 'weights':
      filled = Object.fromEntries(taking.map((id) => [id, '1']))
      break
    case 'exact':
      if (shares) filled = Object.fromEntries(taking.map((id) => [id, formatAmountInput(shares.get(id) ?? 0, currency)]))
      break
    case 'percentage': {
      const parts = hundredths(taking.map((id) => shares?.get(id) ?? 1))
      filled = Object.fromEntries(taking.map((id, i) => [id, formatPercentInput(parts[i] ?? 0)]))
      break
    }
  }
  return { values: filled ? { ...values, [to]: filled } : values, included: ticked }
}

// Each weight's share of 100 %, in hundredths that add up to exactly 10000: each
// is rounded down and the hundredths left go to the largest remainders, the first
// one on a tie. All weights zero count as equal.
export function hundredths(weights: readonly number[]): number[] {
  const all = weights.map((w) => BigInt(w))
  const sum = all.reduce((a, b) => a + b, 0n)
  const parts = sum > 0n ? all : all.map(() => 1n)
  const total = sum > 0n ? sum : BigInt(all.length)
  const shares = parts.map((w, index) => ({ index, value: (w * 10000n) / total, rest: (w * 10000n) % total }))
  let left = 10000n - shares.reduce((a, x) => a + x.value, 0n)
  const byRest = [...shares].sort((a, b) => (b.rest > a.rest ? 1 : b.rest < a.rest ? -1 : a.index - b.index))
  for (const share of byRest) {
    if (left <= 0n) break
    share.value += 1n
    left -= 1n
  }
  return shares.map((x) => Number(x.value))
}

export function initialPaidText(draft?: ExpenseDraft): Record<string, string> {
  if (!draft) return {}
  return Object.fromEntries(draft.paidShares.map(({ memberId, amount }) => [memberId, formatAmountInput(amount, draft.currency)]))
}

// The Cuotas exactly as the domain computed them, those that round down to zero
// included, so that re-editing keeps the split.
export function toOwedShares(shares: readonly SplitShare[]): OwedShare[] {
  return shares.map(({ participantId, amount, splitValue }) => ({
    memberId: participantId,
    amount,
    ...(splitValue === null ? {} : { splitValue }),
  }))
}

// An Expense as the Balances see it: who paid and who owes, in which currency.
export function balanceRecords({ currency, paidShares, owedShares }: Pick<NewExpense, 'currency' | 'paidShares' | 'owedShares'>): Records {
  return { expenses: [expenseRecord({ currency, paidShares, owedShares })], settlements: [] }
}

// What the former members have in an Expense, which cannot change
// (docs/reglas-de-negocio.md). An OwedShare of zero counts as taking part.
export interface FormerShares {
  paid: ReadonlyMap<string, number>
  owed: ReadonlyMap<string, OwedShare>
}

export const noFormerShares: FormerShares = { paid: new Map(), owed: new Map() }

export function formerShares(draft: ExpenseDraft, members: readonly Member[]): FormerShares {
  const former = new Set(members.filter(({ status }) => status === 'former').map(({ id }) => id))
  return {
    paid: new Map(draft.paidShares.filter(({ memberId }) => former.has(memberId)).map(({ memberId, amount }) => [memberId, amount])),
    owed: new Map(draft.owedShares.filter(({ memberId }) => former.has(memberId)).map((share) => [share.memberId, share])),
  }
}

// A former member's Cuota is only known in the method it was saved with, and as
// an amount in exact. Any other method would have to invent their value.
export function methodsFor(saved: SplitMethod | undefined, former: FormerShares): readonly SplitMethod[] {
  if (former.owed.size === 0 || saved === undefined) return splitMethods
  return splitMethods.filter((method) => method === saved || method === 'exact')
}

// The former member's row in the split: always in, with what was saved.
export function formerParticipant({ memberId, amount, splitValue }: OwedShare, method: SplitMethod): SplitParticipant {
  return { memberId, included: true, value: method === 'exact' ? amount : (splitValue ?? 0) }
}

export interface ChangedShare {
  memberId: string
  from: number
  to: number
}

// The first former member whose Cuota this split would change. With equal shares
// any new total or Participant does, and the server would refuse it.
export function changedFormerShare(former: FormerShares, shares: readonly SplitShare[]): ChangedShare | undefined {
  for (const { memberId, amount } of former.owed.values()) {
    const to = shares.find(({ participantId }) => participantId === memberId)?.amount ?? 0
    if (to !== amount) return { memberId, from: amount, to }
  }
  return undefined
}
