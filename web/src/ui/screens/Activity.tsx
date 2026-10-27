import { useTranslation } from 'react-i18next'
import { Figure } from '../components/icons'
import { Screen } from '../components/Shell'
import { ledgerById, ledgerName, memberById, myLedgers, myMemberId, useDemo } from '../demo/store'
import { formatRelative } from '../dates'
import { href } from '../router'

// Activity of one ledger, or of every Group and Friendship when ledgerId is omitted.
export function ActivityList({ ledgerId }: { ledgerId?: string }) {
  const { t } = useTranslation()
  const s = useDemo()
  const visible = new Set(myLedgers(s).map((l) => l.id))
  const items = s.activities
    .filter((a) => (ledgerId ? a.ledgerId === ledgerId : visible.has(a.ledgerId)))
    .sort((a, b) => b.at.localeCompare(a.at))

  if (items.length === 0) return <p className="empty">{ledgerId ? t('ledger.emptyActivity') : t('activity.empty')}</p>

  return (
    <ul className="activity">
      {items.map((a) => {
        const actor = memberById(s, a.actorMemberId)
        const isMe = a.actorMemberId === myMemberId(s, a.ledgerId)
        const ledger = ledgerById(s, a.ledgerId)
        return (
          <li key={a.id}>
            <Figure state={actor?.userId ? 'linked' : 'pending'} size={22} />
            <p>
              <span className="activity-text">{isMe ? t(`activityMine.${a.kind}`, { subject: a.subject }) : t(`activity.${a.kind}`, { actor: actor?.name, subject: a.subject })}</span>
              <span className="activity-meta">
                {formatRelative(a.at)}
                {!ledgerId && ledger && (
                  <>
                    {' · '}
                    <a href={href({ name: 'ledger', ledgerId: ledger.id })}>{ledgerName(s, ledger)}</a>
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

export function Activity() {
  const { t } = useTranslation()
  return (
    <Screen tab="activity">
      <h1 className="page-title">{t('activity.title')}</h1>
      <ActivityList />
    </Screen>
  )
}
