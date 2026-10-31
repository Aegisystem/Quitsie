import { minorUnitDigits } from '../../domain'
import type { ExpenseView, LedgerSummary, Member, SettlementView, User } from '../types'

// What the screens work out from the replica's rows. The replica does not know
// who the User is, so each of these is told.

// The User's place in a ledger. Without one, the ledger is not theirs (any more).
export function myMember(members: readonly Member[], userId: string): Member | undefined {
  return members.find((member) => member.userId === userId && member.status === 'active')
}

// The other side of a Friendship, pending or linked.
export function friendOf(members: readonly Member[], userId: string): Member | undefined {
  return members.find((member) => member.userId !== userId)
}

// A Group has a name; a Friendship goes by the Friend's.
export function ledgerName({ ledger, members }: Pick<LedgerSummary, 'ledger' | 'members'>, userId: string): string {
  return ledger.kind === 'group' ? ledger.name : (friendOf(members, userId)?.name ?? '')
}

// Amounts in different currencies are not comparable, so the order is the
// preferred currency first and then by code, never by size.
function inOrder(totals: Map<string, number>, preferredCurrency: string): Map<string, number> {
  return new Map([...totals].sort(([a], [b]) => (a === preferredCurrency ? -1 : b === preferredCurrency ? 1 : a.localeCompare(b))))
}

// The User's Balance in one ledger, per currency, leaving out the settled ones.
export function myBalance({ balances }: Pick<LedgerSummary, 'balances'>, memberId: string, preferredCurrency: string): Map<string, number> {
  const mine = balances.filter(({ participantId }) => participantId === memberId)
  return inOrder(new Map(mine.map(({ currency, amount }) => [currency, amount])), preferredCurrency)
}

// The User's total per currency across every Group and Friendship. Never converted.
export function myTotals(home: readonly LedgerSummary[], user: Pick<User, 'id' | 'preferredCurrency'>): Map<string, number> {
  const totals = new Map<string, number>()
  for (const summary of home) {
    const me = myMember(summary.members, user.id)
    if (!me) continue
    for (const [currency, amount] of myBalance(summary, me.id, user.preferredCurrency)) {
      totals.set(currency, (totals.get(currency) ?? 0) + amount)
    }
  }
  return inOrder(new Map([...totals].filter(([, amount]) => amount !== 0)), user.preferredCurrency)
}

export interface BalanceProofLine {
  id: string
  description: string
  date: string
  currency: string
  // Effect on the Member's Balance.
  amount: number
}

// What makes up one Member's Balance: every Expense and Settlement that moves it.
// Deleted and rejected records do not count, as in the Balance itself.
export function balanceProof(
  memberId: string,
  expenses: readonly ExpenseView[],
  settlements: readonly SettlementView[],
  names: (memberId: string) => string,
): BalanceProofLine[] {
  const counts = ({ deletedAt, sync }: { deletedAt?: string; sync: string }) => !deletedAt && sync !== 'rejected'
  const lines: BalanceProofLine[] = []
  for (const expense of expenses.filter(counts)) {
    const paid = expense.paidShares.filter((share) => share.memberId === memberId).reduce((sum, share) => sum + share.amount, 0)
    const owed = expense.owedShares.filter((share) => share.memberId === memberId).reduce((sum, share) => sum + share.amount, 0)
    if (paid === 0 && owed === 0) continue
    const { id, description, date, currency } = expense
    lines.push({ id, description, date, currency, amount: paid - owed })
  }
  for (const { id, fromMemberId, toMemberId, date, currency, amount } of settlements.filter(counts)) {
    if (fromMemberId !== memberId && toMemberId !== memberId) continue
    const description = `${names(fromMemberId)} → ${names(toMemberId)}`
    lines.push({ id, description, date, currency, amount: fromMemberId === memberId ? amount : -amount })
  }
  return lines.sort((a, b) => b.date.localeCompare(a.date))
}

// The same number in another currency, as changing the Group currency relabels
// it: 10,00 USD is 10 CLP. Undefined when it would lose minor units.
export function relabelAmount(amount: number, from: string, to: string): number | undefined {
  const digits = minorUnitDigits(to) - minorUnitDigits(from)
  if (digits >= 0) return amount * 10 ** digits
  const lost = 10 ** -digits
  return amount % lost === 0 ? amount / lost : undefined
}
