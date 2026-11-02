import { formatMoney } from '../money'
import type { Activity } from '../types'

// The sentence an Activity reads as, with the values that fill it in. `details`
// depends on `kind`; a kind this version does not know yet is skipped, as the
// contract asks.

export type ActivitySentence =
  | 'group_created'
  | 'group_renamed'
  | 'group_currency_changed'
  | 'group_deleted'
  | 'debt_simplification_on'
  | 'debt_simplification_off'
  | 'member_added'
  | 'member_updated'
  | 'member_joined'
  | 'member_claimed'
  | 'claim_undone'
  | 'member_removed'
  | 'member_left'
  | 'member_readmitted'
  | 'invite_link_regenerated'
  | 'friendship_created'
  | 'expense_created'
  | 'expense_updated'
  | 'expense_deleted'
  | 'expense_restored'
  | 'settlement_created'
  | 'settlement_deleted'
  | 'settlement_restored'

export interface ActivityLine {
  sentence: ActivitySentence
  values: { from?: string; to?: string; name?: string; description?: string; amount?: string }
}

const text = (value: unknown) => (typeof value === 'string' ? value : '')

export function describeActivity({ kind, details }: Activity, nameOf: (memberId: string) => string): ActivityLine | undefined {
  const member = () => nameOf(text(details.memberId))

  switch (kind) {
    case 'group_created':
    case 'group_deleted':
    case 'member_joined':
    case 'member_claimed':
    case 'member_left':
    case 'invite_link_regenerated':
      return { sentence: kind, values: {} }
    case 'group_renamed':
    case 'group_currency_changed':
    case 'member_updated':
      return { sentence: kind, values: { from: text(details.from), to: text(details.to) } }
    case 'debt_simplification_changed':
      return { sentence: details.simplifyDebts ? 'debt_simplification_on' : 'debt_simplification_off', values: {} }
    case 'member_added':
      return { sentence: kind, values: { name: text(details.name) } }
    case 'claim_undone':
    case 'member_removed':
    case 'member_readmitted':
    case 'friendship_created':
      return { sentence: kind, values: { name: member() } }
    case 'expense_created':
    case 'expense_deleted':
    case 'expense_restored':
      return { sentence: kind, values: { description: text(details.description) } }
    case 'expense_updated': {
      const after = details.after as { description?: unknown } | undefined
      return { sentence: kind, values: { description: text(after?.description) } }
    }
    case 'settlement_created':
    case 'settlement_deleted':
    case 'settlement_restored':
      return {
        sentence: kind,
        values: {
          from: nameOf(text(details.fromMemberId)),
          to: nameOf(text(details.toMemberId)),
          amount: formatMoney(Number(details.amount), text(details.currency)),
        },
      }
    default:
      return undefined
  }
}
