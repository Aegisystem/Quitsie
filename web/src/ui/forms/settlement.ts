import type { TFunction } from 'i18next'
import type { Member, NewSettlement } from '../../api/client'
import type { RejectionReason } from '../../local'

// A RejectedRecord the Settlement form corrects, as the Expense form does with
// an Expense (ADR 0001).
export interface SettlementFix {
  id: string
  reason: RejectionReason
  draft: Omit<NewSettlement, 'id' | 'ledgerId'>
}

// Who the form starts with. A suggested Debt may be older than someone leaving,
// so it falls back to active Members. A record to fix keeps who it had, even if
// they left: the form then says that only active Members can take part, instead
// of putting someone else in without the User noticing.
export function initialPeople(
  active: readonly Member[],
  me: Member,
  { from, to, fix }: { from?: string; to?: string; fix?: SettlementFix },
): { from: string; to: string } {
  if (fix) return { from: fix.draft.fromMemberId, to: fix.draft.toMemberId }
  const activeId = (id?: string) => active.find((m) => m.id === id)?.id
  const first = activeId(from) ?? me.id
  return { from: first, to: activeId(to) ?? active.find((m) => m.id !== first)?.id ?? '' }
}

// Why the server refused the Settlement, and what to look at before saving it again.
export function settlementRejectionText({ reason, draft }: SettlementFix, groupCurrency: string | undefined, t: TFunction): string {
  if (reason === 'currency_mismatch' && groupCurrency) return t('settlement.rejectedCurrency', { from: draft.currency, to: groupCurrency })
  if (reason === 'invalid_participants') return t('settlement.rejectedParticipants')
  return t('settlement.rejectedOther')
}
