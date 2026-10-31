import type { Member } from '../../api/client'
import type { LedgerSummary } from '../../local'
import type { FigureState } from '../components/icons'

export const isActive = (member: Member) => member.status === 'active'

// The replica does not know who the User is, so the screens find their Member.
export function myMember(members: readonly Member[], userId: string): Member | undefined {
  return members.find((member) => member.userId === userId && isActive(member))
}

// Who can take part in something new, with the User first.
export function activeMembers(members: readonly Member[], me: Member): Member[] {
  return [me, ...members.filter((member) => isActive(member) && member.id !== me.id)]
}

export function figureState(member: Member): FigureState {
  if (!isActive(member)) return 'former'
  return member.userId ? 'linked' : 'pending'
}

// A Group has its name; a Friendship goes by the Friend's.
export function ledgerName({ ledger, members }: Pick<LedgerSummary, 'ledger' | 'members'>, userId: string): string {
  return ledger.kind === 'group' ? ledger.name : (members.find((member) => member.userId !== userId)?.name ?? '')
}
