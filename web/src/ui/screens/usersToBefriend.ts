import type { LedgerSummary } from '../../local'

export interface UserToBefriend {
  userId: string
  name: string
  groups: string[]
}

// A Friendship with someone who already has an account can only start directly
// if the two share a Group (docs/reglas-de-negocio.md), so these are the Users
// in the User's Groups who are not their Friends yet, by name.
export function usersToBefriend(home: LedgerSummary[], myUserId: string): UserToBefriend[] {
  const active = (summary: LedgerSummary) => summary.members.filter((member) => member.status === 'active')
  const friends = new Set(home.filter(({ ledger }) => ledger.kind === 'friendship').flatMap((summary) => summary.members.map((member) => member.userId)))

  const found = new Map<string, UserToBefriend>()
  for (const summary of home) {
    const { ledger } = summary
    if (ledger.kind !== 'group' || !active(summary).some((member) => member.userId === myUserId)) continue
    for (const { userId, name } of active(summary)) {
      if (!userId || userId === myUserId || friends.has(userId)) continue
      const user = found.get(userId) ?? { userId, name, groups: [] }
      user.groups.push(ledger.name)
      found.set(userId, user)
    }
  }
  return [...found.values()].sort((a, b) => a.name.localeCompare(b.name))
}
