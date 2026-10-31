import type { Member, NewExpense, OwedShare, SplitMethod } from '../../api/client'
import type { OwedShare as SplitShare } from '../../domain'
import { formatAmountInput, formatPercentInput } from '../money'
import type { SplitParticipant } from '../split'
import { splitMethods } from '../types'

// What the form starts from when it is not blank: a RejectedRecord to fix or an
// Expense to edit.
export type ExpenseDraft = Omit<NewExpense, 'id' | 'ledgerId'>

// What the Usuario typed for each Member, kept per method so that going through
// the methods loses nothing.
export type SplitValues = Record<SplitMethod, Record<string, string>>

export function initialSplitValues(memberIds: readonly string[], draft?: ExpenseDraft): SplitValues {
  const values: SplitValues = { equal: {}, exact: {}, percentage: {}, weights: {}, adjustment: {} }
  // Someone without a weight stayed out of the split, so a draft keeps them out.
  if (draft?.splitMethod !== 'weights') for (const id of memberIds) values.weights[id] = '1'
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
