import { useTranslation } from 'react-i18next'
import { Amount, BalanceBands, toneOf } from '../components/Balance'
import { Figure, GroupGlyph, Icon } from '../components/icons'
import { InstallPrompt, Screen, toast, useOnline } from '../components/Shell'
import { ledgerMembers, ledgerName, memberById, myLedgerBalance, myLedgers, myTotals, useDemo } from '../demo/store'
import type { Ledger } from '../demo/types'
import { href } from '../router'

export function Home() {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const ledgers = myLedgers(s)
  const groups = ledgers.filter((l) => l.kind === 'group')
  const friends = ledgers.filter((l) => l.kind === 'friendship')

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
        <BalanceBands balances={myTotals(s)} settledTitle={t('balance.settledAll')} settledBody={t('balance.settledAllWink')} />
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
          <button type="button" className="button button-small button-quiet" disabled={!online} title={online ? undefined : t('common.onlineOnly')} onClick={() => toast(t('home.notInDemo'))}>
            <Icon name="plus" size={18} />
            {t('home.newGroup')}
          </button>
        </div>
        {groups.length === 0 ? <p className="empty">{t('home.emptyGroups')}</p> : <LedgerList ledgers={groups} />}
      </section>

      <section className="section" aria-labelledby="friends-title">
        <div className="section-head">
          <h2 id="friends-title" className="section-title">
            {t('home.friends')}
          </h2>
          <button type="button" className="button button-small button-quiet" disabled={!online} title={online ? undefined : t('common.onlineOnly')} onClick={() => toast(t('home.notInDemo'))}>
            <Icon name="plus" size={18} />
            {t('home.newFriend')}
          </button>
        </div>
        {friends.length === 0 ? <p className="empty">{t('home.emptyFriends')}</p> : <LedgerList ledgers={friends} />}
      </section>
      </div>

      <InstallPrompt />
    </Screen>
  )
}

function LedgerList({ ledgers }: { ledgers: Ledger[] }) {
  const { t } = useTranslation()
  const s = useDemo()
  return (
    <ul className="rows">
      {ledgers.map((ledger) => {
        const balances = [...myLedgerBalance(s, ledger.id)]
        const friend = ledger.kind === 'friendship' ? memberById(s, ledger.friendMemberId) : undefined
        const pending = friend && !friend.userId
        const sub =
          ledger.kind === 'group'
            ? t('ledger.members', { count: ledgerMembers(s, ledger.id).filter((m) => m.status === 'active').length })
            : pending
              ? t('ledger.pending')
              : undefined
        return (
          <li key={ledger.id}>
            <a className="row row-link" href={href({ name: 'ledger', ledgerId: ledger.id })}>
              <span className="row-glyph">{ledger.kind === 'group' ? <GroupGlyph /> : <Figure state={pending ? 'pending' : 'linked'} />}</span>
              <span className="row-main">
                <span className="row-title">{ledgerName(s, ledger)}</span>
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
