import { useTranslation } from 'react-i18next'
import { Figure, figureState } from '../components/icons'
import { Screen } from '../components/Shell'
import { describeActivity, ledgerName, replica, useLive } from '../data'
import { formatRelative } from '../dates'
import { liveActivity, liveHome } from '../../local'
import { href } from '../router'
import type { User } from '../types'

// Activity of one ledger, or of every Group and Friendship when ledgerId is omitted.
export function ActivityList({ ledgerId, user }: { ledgerId?: string; user: User }) {
  const { t } = useTranslation()
  const [activities] = useLive(`activity:${ledgerId ?? 'all'}`, () => liveActivity(replica, ledgerId))
  const [home] = useLive('home', () => liveHome(replica))
  if (!activities || !home) return null

  const members = home.flatMap((summary) => summary.members)
  const nameOf = (memberId: string) => {
    const member = members.find(({ id }) => id === memberId)
    return member?.userId === user.id ? t('common.you') : (member?.name ?? '?')
  }
  const lines = activities.flatMap((activity) => {
    const line = describeActivity(activity, nameOf)
    return line ? [{ activity, ...line }] : []
  })

  if (lines.length === 0) return <p className="empty">{ledgerId ? t('ledger.emptyActivity') : t('activity.empty')}</p>

  return (
    <ul className="activity">
      {lines.map(({ activity, sentence, values }) => {
        const summary = home.find(({ ledger }) => ledger.id === activity.ledgerId)
        const actor = summary?.members.find(({ userId }) => userId === activity.userId)
        return (
          <li key={activity.id}>
            <Figure state={figureState(actor)} size={22} />
            <p>
              <span className="activity-text">
                {activity.userId === user.id
                  ? t(`activityLogMine.${sentence}`, values)
                  : t(`activityLog.${sentence}`, { ...values, actor: actor?.name ?? t('activityLog.someone') })}
              </span>
              <span className="activity-meta">
                {formatRelative(activity.createdAt)}
                {!ledgerId && summary && (
                  <>
                    {' · '}
                    <a href={href({ name: 'ledger', ledgerId: activity.ledgerId })}>{ledgerName(summary, user.id)}</a>
                  </>
                )}
              </span>
            </p>
          </li>
        )
      })}
    </ul>
  )
}

export function Activity({ user }: { user: User }) {
  const { t } = useTranslation()
  return (
    <Screen tab="activity">
      <h1 className="page-title">{t('activity.title')}</h1>
      <ActivityList user={user} />
    </Screen>
  )
}
