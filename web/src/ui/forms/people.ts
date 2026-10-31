import type { Member } from '../../api/client'

export const isActive = (member: Member) => member.status === 'active'

// Who can take part in something new, with the User first.
export function activeMembers(members: readonly Member[], me: Member): Member[] {
  return [me, ...members.filter((member) => isActive(member) && member.id !== me.id)]
}
