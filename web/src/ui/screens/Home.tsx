import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { liveHome, liveRecords } from '../../local'
import { Amount, BalanceBands, toneOf } from '../components/Balance'
import { Figure, figureState, GroupGlyph, Icon } from '../components/icons'
import { FirstSync, InstallPrompt, Screen, useOnline } from '../components/Shell'
import { friendOf, ledgerName, myBalance, myMember, myTotals, replica, useLive, useReplicaFilled } from '../data'
import { href } from '../router'
import type { LedgerSummary, User } from '../types'

// Those screens are built apart, so the addresses are written out here.
const newGroupHref = '#/groups/new'
const newFriendHref = '#/friends/new'

// Creating Groups and Friends goes straight to the API (ADR 0001).
function CreateLink({ to, className, children }: { to: string; className: string; children: ReactNode }) {
  const { t } = useTranslation()
  const online = useOnline()
  return (
    <a className={`button ${className}`} href={online ? to : undefined} aria-disabled={!online} title={online ? undefined : t('common.onlineOnly')}>
      {children}
    </a>
  )
}

export function Home({ user }: { user: User }) {
  const { t } = useTranslation()
  const online = useOnline()
  const [home] = useLive('home', () => liveHome(replica))
  const [records] = useLive('records', () => liveRecords(replica))
  const filled = useReplicaFilled()

  const mine = (home ?? []).filter(({ members }) => myMember(members, user.id))
  const byName = (a: LedgerSummary, b: LedgerSummary) => ledgerName(a, user.id).localeCompare(ledgerName(b, user.id))
  const groups = mine.filter(({ ledger }) => ledger.kind === 'group').sort(byName)
  const friends = mine.filter(({ ledger }) => ledger.kind === 'friendship').sort(byName)

  if (mine.length === 0) {
    return (
      <Screen tab="home" wide className="screen-home">
        <h1 className="sr-only">{t('home.title')}</h1>
        {home && filled === false && <FirstSync />}
        {home && filled && (
          <section className="first-run">
            <span className="first-run-figures" aria-hidden="true">
              <Figure size={40} />
              <Figure state="pending" size={40} />
              <Figure state="pending" size={40} />
            </span>
            <h2 className="page-title">{t('home.firstRunTitle')}</h2>
            <p className="lead">{t('home.firstRunBody')}</p>
            <div className="first-run-actions">
              <CreateLink to={newGroupHref} className="button-primary">
                <Icon name="plus" />
                {t('home.newGroup')}
              </CreateLink>
              <CreateLink to={newFriendHref} className="button-secondary">
                <Icon name="plus" />
                {t('home.newFriend')}
              </CreateLink>
            </div>
            {!online && <p className="hint">{t('common.onlineOnly')}</p>}
          </section>
        )}
        <InstallPrompt />
      </Screen>
    )
  }

  return (
    <Screen
      tab="home"
      wide
      className="screen-home"
      dock={
        <a className="button button-primary button-block" href={href({ name: 'newExpense' })}>
          <Icon name="plus" />
          {t('home.addExpense')}
        </a>
      }
    >
      <h1 className="sr-only">{t('home.title')}</h1>
      <section aria-label={t('home.summary')} className="full-bleed">
        <BalanceBands
          balances={myTotals(mine, user)}
          settledTitle={t('balance.settledAll')}
          settledBody={t('balance.settledAllWink')}
          provisional={records?.unsynced.some(({ ledgerId }) => mine.some(({ ledger }) => ledger.id === ledgerId))}
        />
      </section>

      <div className="home-actions">
        <a className="button button-primary" href={href({ name: 'newExpense' })}>
          <Icon name="plus" />
          {t('home.addExpense')}
        </a>
      </div>

      <div className="home-lists">

      <section className="section" aria-labelledby="groups-title">
        <div className="section-head">
          <h2 id="groups-title" className="section-title">
            {t('home.groups')}
          </h2>
          <CreateLink to={newGroupHref} className="button-small button-quiet">
            <Icon name="plus" size={18} />
            {t('home.newGroup')}
          </CreateLink>
        </div>
        {groups.length === 0 ? <p className="empty">{t('home.emptyGroups')}</p> : <LedgerList ledgers={groups} user={user} />}
      </section>

      <section className="section" aria-labelledby="friends-title">
        <div className="section-head">
          <h2 id="friends-title" className="section-title">
            {t('home.friends')}
          </h2>
          <CreateLink to={newFriendHref} className="button-small button-quiet">
            <Icon name="plus" size={18} />
            {t('home.newFriend')}
          </CreateLink>
        </div>
        {friends.length === 0 ? <p className="empty">{t('home.emptyFriends')}</p> : <LedgerList ledgers={friends} user={user} />}
      </section>
      </div>

      <InstallPrompt />
    </Screen>
  )
}

function LedgerList({ ledgers, user }: { ledgers: LedgerSummary[]; user: User }) {
  const { t } = useTranslation()
  return (
    <ul className="rows">
      {ledgers.map((summary) => {
        const { ledger, members } = summary
        const me = myMember(members, user.id)
        const balances = me ? [...myBalance(summary, me.id, user.preferredCurrency)] : []
        const friend = ledger.kind === 'friendship' ? friendOf(members, user.id) : undefined
        const pending = friend && !friend.userId
        const sub =
          ledger.kind === 'group'
            ? t('ledger.members', { count: members.filter((m) => m.status === 'active').length })
            : pending
              ? t('ledger.pending')
              : undefined
        return (
          <li key={ledger.id}>
            <a className="row row-link" href={href({ name: 'ledger', ledgerId: ledger.id })}>
              <span className="row-glyph">{ledger.kind === 'group' ? <GroupGlyph /> : <Figure state={figureState(friend)} />}</span>
              <span className="row-main">
                <span className="row-title">{ledgerName(summary, user.id)}</span>
                {sub && <span className="row-sub">{sub}</span>}
              </span>
              <span className="row-end">
                {balances.length === 0 ? (
                  <span className="settled-tag">
                    <Icon name="check" size={16} />
                    {t('balance.memberSettled')}
                  </span>
                ) : (
                  balances.map(([currency, amount]) => (
                    <span key={currency} className="row-balance">
                      <span className={`row-balance-label tone-${toneOf(amount)}`}>{amount > 0 ? t('balance.owedToYou') : t('balance.youOwe')}</span>
                      <Amount minor={Math.abs(amount)} currency={currency} tone={toneOf(amount)} />
                    </span>
                  ))
                )}
              </span>
            </a>
          </li>
        )
      })}
    </ul>
  )
}
