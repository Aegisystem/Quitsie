import type { NewExpense, NewSettlement } from '../api/client'
import type { ExpenseRecord, SettlementRecord } from '../domain'

// An Expense or a Settlement as the domain adds it up into Balances, where each
// Member is a Participant. The provisional Balances and the forms' check against
// them both go through here, so they always count a record the same way. A
// record not saved yet has no id.

type Optional = { id?: string; deletedAt?: string }

export function expenseRecord({
  id = '',
  currency,
  paidShares,
  owedShares,
  deletedAt,
}: Pick<NewExpense, 'currency' | 'paidShares' | 'owedShares'> & Optional): ExpenseRecord {
  const share = ({ memberId, amount }: { memberId: string; amount: number }) => ({ participantId: memberId, amount })
  return { id, currency, paidShares: paidShares.map(share), owedShares: owedShares.map(share), deletedAt }
}

export function settlementRecord({
  id = '',
  fromMemberId,
  toMemberId,
  amount,
  currency,
  deletedAt,
}: Pick<NewSettlement, 'fromMemberId' | 'toMemberId' | 'amount' | 'currency'> & Optional): SettlementRecord {
  return { id, from: fromMemberId, to: toMemberId, amount, currency, deletedAt }
}
