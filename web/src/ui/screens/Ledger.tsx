import { useTranslation } from 'react-i18next'
import { Amount, BalanceBands, SyncMark, Tally, TallyLegend, toneOf } from '../components/Balance'
import { CategoryGlyph, Figure, Icon, SettlementGlyph } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { ActivityList } from './Activity'
import { balanceProof, pairwiseDebts, simplifyDebts, type Debt } from '../demo/balances'
import {
  demo,
  ledgerBalances,
  ledgerById,
  ledgerExpenses,
  ledgerMembers,
  ledgerName,
  ledgerSettlements,
  memberById,
  myLedgerBalance,
  myMemberId,
  useDemo,
  type DemoState,
} from '../demo/store'
import type { Expense, Group, Ledger as LedgerT, Member, Settlement } from '../demo/types'
import { formatDay } from '../dates'
import { formatMoney, tallyUnit } from '../money'
import { href } from '../router'

type Tab = 'expenses' | 'balances' | 'debts' | 'activity'

export function Ledger({ ledgerId, tab }: { ledgerId: string; tab?: string }) {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const ledger = ledgerById(s, ledgerId)
  const me = ledger && myMemberId(s, ledger.id)

  if (!ledger || !me) {
    return (
      <Screen back={{ name: 'home' }} title={t('nav.home')}>
        <p className="empty">{t('ledger.unknown')}</p>
      </Screen>
    )
  }

  const tabs: Tab[] = ledger.kind === 'group' ? ['expenses', 'balances', 'debts', 'activity'] : ['expenses', 'activity']
  const current: Tab = tabs.includes(tab as Tab) ? (tab as Tab) : 'expenses'
  const mine = myLedgerBalance(s, ledger.id)
  const name = ledgerName(s, ledger)
  const friend = ledger.kind === 'friendship' ? memberById(s, ledger.friendMemberId) : undefined
  const canInvite = ledger.kind === 'group' || (friend && !friend.userId)
  const settleRoute = firstSettlement(s, ledger, me)

  return (
    <Screen
      back={{ name: 'home' }}
      title={name}
      actions={
        canInvite && (
          <a
            className={`icon-button${online ? '' : ' is-disabled'}`}
            href={online ? href({ name: 'inviteLink', ledgerId: ledger.id }) : undefined}
            aria-disabled={!online}
            aria-label={t('ledger.invite')}
            title={online ? t('ledger.invite') : t('common.onlineOnly')}
          >
            <Icon name="share" />
          </a>
        )
      }
      dock={
        <>
          <a className="button button-secondary" href={href(settleRoute)}>
            {t('ledger.settle')}
          </a>
          <a className="button button-primary button-grow" href={href({ name: 'newExpense', ledgerId: ledger.id })}>
            <Icon name="plus" />
            {t('ledger.addExpense')}
          </a>
        </>
      }
    >
      <div className="full-bleed">
        <BalanceBands
          balances={mine}
          settledBody={t('ledger.noDebtsWink')}
          footer={
            ledger.kind === 'group' && (
              <a className="band-link" href={href({ name: 'ledger', ledgerId: ledger.id, tab: 'balances' })}>
                {t('ledger.proof')}
                <Icon name="chevron" size={18} />
              </a>
            )
          }
        />
      </div>

      <p className="ledger-meta">
        {ledger.kind === 'group'
          ? `${t('ledger.groupCurrency', { currency: ledger.currency })} · ${t('ledger.members', { count: ledgerMembers(s, ledger.id).filter((m) => m.status === 'active').length })}`
          : t('ledger.friendship')}
        {friend && !friend.userId && <span className="tag tag-pending">{t('ledger.pending')}</span>}
      </p>

      <nav className="tabs" aria-label={t('ledger.sections')}>
        {tabs.map((id) => (
          <a key={id} href={href({ name: 'ledger', ledgerId: ledger.id, tab: id })} aria-current={current === id ? 'page' : undefined}>
            {t(`ledger.tabs.${id}`)}
          </a>
        ))}
      </nav>

      <div className="tab-panel">
        {current === 'expenses' && <ExpensesTab ledger={ledger} me={me} />}
        {current === 'balances' && ledger.kind === 'group' && <BalancesTab group={ledger} me={me} />}
        {current === 'debts' && ledger.kind === 'group' && <DebtsTab group={ledger} me={me} />}
        {current === 'activity' && <ActivityList ledgerId={ledger.id} />}
      </div>
    </Screen>
  )
}

// Where "Liquidar" starts: the first suggested Debt that involves the User, if any.
function firstSettlement(s: DemoState, ledger: LedgerT, me: string) {
  if (ledger.kind === 'friendship') {
    const [currency, amount] = [...myLedgerBalance(s, ledger.id)][0] ?? []
    if (currency && amount) {
      const friend = ledger.friendMemberId
      return amount < 0
        ? { name: 'settle' as const, ledgerId: ledger.id, from: me, to: friend, amount: String(-amount), currency }
        : { name: 'settle' as const, ledgerId: ledger.id, from: friend, to: me, amount: String(amount), currency }
    }
    return { name: 'settle' as const, ledgerId: ledger.id }
  }
  const debt = groupDebts(s, ledger).find((d) => d.fromMemberId === me || d.toMemberId === me)
  return debt
    ? { name: 'settle' as const, ledgerId: ledger.id, from: debt.fromMemberId, to: debt.toMemberId, amount: String(debt.amount) }
    : { name: 'settle' as const, ledgerId: ledger.id }
}

function groupDebts(s: DemoState, group: Group): Debt[] {
  if (group.simplifyDebts) {
    const byMember = new Map<string, number>()
    for (const [memberId, byCurrency] of ledgerBalances(s, group.id)) byMember.set(memberId, byCurrency.get(group.currency) ?? 0)
    return simplifyDebts(byMember)
  }
  return pairwiseDebts(ledgerExpenses(s, group.id), ledgerSettlements(s, group.id), group.currency)
}

function useNames(s: DemoState, me: string) {
  const { t } = useTranslation()
  return (memberId: string) => (memberId === me ? t('common.you') : (memberById(s, memberId)?.name ?? '?'))
}

type Item = { kind: 'expense'; date: string; at: string; expense: Expense } | { kind: 'settlement'; date: string; at: string; settlement: Settlement }

function ExpensesTab({ ledger, me }: { ledger: LedgerT; me: string }) {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const name = useNames(s, me)

  const items: Item[] = [
    ...ledgerExpenses(s, ledger.id).map((expense) => ({ kind: 'expense' as const, date: expense.date, at: expense.createdAt, expense })),
    ...ledgerSettlements(s, ledger.id).map((settlement) => ({ kind: 'settlement' as const, date: settlement.date, at: settlement.createdAt, settlement })),
  ].sort((a, b) => b.date.localeCompare(a.date) || b.at.localeCompare(a.at))

  if (items.length === 0) return <p className="empty">{t('ledger.emptyExpenses')}</p>

  const days = [...new Set(items.map((i) => i.date))]

  return (
    <div className="day-list">
      {days.map((day) => (
        <section key={day} aria-label={formatDay(day)}>
          <h3 className="day-title">{formatDay(day)}</h3>
          <ul className="rows">
            {items
              .filter((i) => i.date === day)
              .map((item) =>
                item.kind === 'expense' ? (
                  <ExpenseRow key={item.expense.id} expense={item.expense} me={me} online={online} name={name} />
                ) : (
                  <li key={item.settlement.id} className="row row-settlement">
                    <span className="row-glyph">
                      <SettlementGlyph />
                    </span>
                    <span className="row-main">
                      <span className="row-title">
                        {t('ledger.settlementLine', {
                          from: name(item.settlement.fromMemberId),
                          to: name(item.settlement.toMemberId),
                          amount: formatMoney(item.settlement.amount, item.settlement.currency),
                        })}
                      </span>
                      <span className="row-sub">{t('ledger.settlementLabel')}</span>
                    </span>
                    <span className="row-end" />
                    <SyncMark state={item.settlement.sync} />
                  </li>
                ),
              )}
          </ul>
        </section>
      ))}
    </div>
  )
}

function ExpenseRow({ expense, me, online, name }: { expense: Expense; me: string; online: boolean; name: (id: string) => string }) {
  const { t } = useTranslation()
  const paid = expense.paidShares.find((p) => p.memberId === me)?.amount ?? 0
  const owed = expense.owedShares.find((o) => o.memberId === me)?.amount ?? 0
  const net = paid - owed
  const payers = expense.paidShares
  const payer =
    payers.length > 1 ? t('ledger.paidByMany', { count: payers.length }) : payers[0]?.memberId === me ? t('ledger.paidByYou') : t('ledger.paidBy', { name: name(payers[0]?.memberId ?? '') })

  let share: { text: string; tone: string } = { text: t('ledger.notInvolved'), tone: 'quiet' }
  if (paid === 0 && owed > 0) share = { text: t('ledger.yourShare', { amount: formatMoney(owed, expense.currency) }), tone: 'against' }
  else if (net > 0) share = { text: t('ledger.youLent', { amount: formatMoney(net, expense.currency) }), tone: 'favor' }
  else if (net < 0) share = { text: t('ledger.yourShare', { amount: formatMoney(-net, expense.currency) }), tone: 'against' }

  if (expense.deletedAt) {
    return (
      <li className="row row-deleted">
        <span className="row-glyph">
          <CategoryGlyph category={expense.category} />
        </span>
        <span className="row-main">
          <span className="row-title">
            <s>{expense.description}</s>
          </span>
          <span className="row-sub">{t('ledger.deleted')}</span>
        </span>
        <span className="row-end">
          <button
            type="button"
            className="button button-small button-quiet"
            disabled={!online}
            title={online ? undefined : t('common.onlineOnly')}
            onClick={() => {
              demo.restoreExpense(expense.id)
              toast(`${t('ledger.restore')}: ${expense.description}`)
            }}
          >
            <Icon name="restore" size={18} />
            {t('ledger.restore')}
          </button>
        </span>
        <span className="sync-cell" />
      </li>
    )
  }

  return (
    <li className="row">
      <span className="row-glyph">
        <CategoryGlyph category={expense.category} />
      </span>
      <span className="row-main">
        <span className="row-title">{expense.description}</span>
        <span className="row-sub">
          {payer} · <span className={`tone-${share.tone}`}>{share.text}</span>
        </span>
      </span>
      <span className="row-end">
        <Amount minor={expense.total} currency={expense.currency} />
      </span>
      <SyncMark state={expense.sync} />
    </li>
  )
}

function figureState(m: Member | undefined) {
  if (!m) return 'linked' as const
  return m.status === 'former' ? ('former' as const) : m.userId ? ('linked' as const) : ('pending' as const)
}

function BalancesTab({ group, me }: { group: Group; me: string }) {
  const { t } = useTranslation()
  const s = useDemo()
  const name = useNames(s, me)
  const balances = ledgerBalances(s, group.id)
  const members = ledgerMembers(s, group.id)
    .filter((m) => m.status === 'active' || (balances.get(m.id)?.get(group.currency) ?? 0) !== 0 || m.status === 'former')
    .sort((a, b) => (a.id === me ? -1 : b.id === me ? 1 : 0))
  const amounts = members.map((m) => balances.get(m.id)?.get(group.currency) ?? 0)
  const unit = tallyUnit(Math.max(1, ...amounts.map(Math.abs)), group.currency, 9)
  const expenses = ledgerExpenses(s, group.id)
  const settlements = ledgerSettlements(s, group.id)

  return (
    <>
      <div className="chart-legend">
        <p>{t('ledger.balancesLegend')}</p>
        <TallyLegend unit={unit} currency={group.currency} />
      </div>
      <ul className="chart">
        {members.map((m, i) => {
          const amount = amounts[i] ?? 0
          const tone = toneOf(amount)
          const proof = balanceProof(m.id, expenses, settlements, name)
          return (
            <li key={m.id} className={`chart-row${m.status === 'former' ? ' is-former' : ''}`}>
              <div className="chart-head">
                <Figure state={figureState(m)} size={24} />
                <span className="chart-name">
                  {name(m.id)}
                  {figureState(m) === 'pending' && <span className="tag tag-pending">{t('ledger.pending')}</span>}
                  {m.status === 'former' && <span className="tag">{t('ledger.former')}</span>}
                </span>
                <span className="chart-amount">
                  <span className={`chart-amount-label tone-${tone}`}>
                    {tone === 'favor' ? t('balance.memberOwed') : tone === 'against' ? t('balance.memberOwes') : t('balance.memberSettled')}
                  </span>
                  {amount !== 0 && <Amount minor={Math.abs(amount)} currency={group.currency} tone={tone} />}
                </span>
              </div>
              <div className={`diverge tone-${tone}`} aria-hidden="true">
                <span className="diverge-side diverge-left">{amount < 0 && <Tally amount={amount} unit={unit} align="end" />}</span>
                <span className="diverge-axis">{amount === 0 && <i className="even-dot" />}</span>
                <span className="diverge-side diverge-right">{amount > 0 && <Tally amount={amount} unit={unit} />}</span>
              </div>
              {proof.length > 0 && (
                <details className="proof">
                  <summary>{t('ledger.proof')}</summary>
                  <ul aria-label={t('ledger.proofTitle', { name: name(m.id) })}>
                    {proof.map((line) => (
                      <li key={line.id}>
                        <span className="proof-desc">{line.description}</span>
                        <span className="proof-date">{formatDay(line.date)}</span>
                        <Amount minor={line.amount} currency={line.currency} signed tone={toneOf(line.amount)} />
                      </li>
                    ))}
                  </ul>
                </details>
              )}
            </li>
          )
        })}
      </ul>
      {members.some((m) => figureState(m) === 'pending') && <p className="hint">{t('ledger.pendingHint')}</p>}
    </>
  )
}

function DebtsTab({ group, me }: { group: Group; me: string }) {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const name = useNames(s, me)
  const debts = groupDebts(s, group)
  const unit = tallyUnit(Math.max(1, ...debts.map((d) => d.amount)), group.currency, 7)

  return (
    <>
      <label className="switch-row">
        <span>
          <span className="switch-title">{t('ledger.simplifyToggle')}</span>
          <span className="switch-hint">{group.simplifyDebts ? t('ledger.simplifyOn') : t('ledger.simplifyOff')}</span>
        </span>
        <input
          type="checkbox"
          role="switch"
          className="switch"
          checked={group.simplifyDebts}
          disabled={!online}
          title={online ? undefined : t('common.onlineOnly')}
          onChange={(e) => demo.setSimplifyDebts(group.id, e.target.checked)}
        />
      </label>

      <h2 className="section-title">{t('ledger.debtsTitle')}</h2>
      {debts.length === 0 ? (
        <div className="empty empty-even">
          <span className="band-figures" aria-hidden="true">
            <Figure size={32} />
            <span className="band-equals">=</span>
            <Figure size={32} />
          </span>
          <p>
            {t('ledger.noDebts')} {t('ledger.noDebtsWink')}
          </p>
        </div>
      ) : (
        <>
          <TallyLegend unit={unit} currency={group.currency} />
          <ul className="rows debts">
            {debts.map((d) => {
              const mine = d.fromMemberId === me || d.toMemberId === me
              const label =
                d.fromMemberId === me
                  ? t('ledger.youPay', { name: name(d.toMemberId) })
                  : d.toMemberId === me
                    ? t('ledger.paysYou', { name: name(d.fromMemberId) })
                    : t('ledger.debtArrow', { from: name(d.fromMemberId), to: name(d.toMemberId) })
              return (
                <li key={`${d.fromMemberId}-${d.toMemberId}`} className={`debt${mine ? ' debt-mine' : ''}`}>
                  <div className="debt-people" aria-hidden="true">
                    <Figure state={figureState(memberById(s, d.fromMemberId))} size={26} />
                    <Icon name="arrow" size={20} />
                    <Figure state={figureState(memberById(s, d.toMemberId))} size={26} />
                  </div>
                  <div className="debt-main">
                    <p className="debt-label">{label}</p>
                    <div className="debt-figure">
                      <Amount minor={d.amount} currency={group.currency} tone={d.fromMemberId === me ? 'against' : d.toMemberId === me ? 'favor' : undefined} />
                      <Tally amount={d.amount} unit={unit} />
                    </div>
                  </div>
                  <a
                    className="button button-small button-secondary"
                    href={href({ name: 'settle', ledgerId: group.id, from: d.fromMemberId, to: d.toMemberId, amount: String(d.amount) })}
                  >
                    {t('ledger.settle')}
                  </a>
                </li>
              )
            })}
          </ul>
        </>
      )}
    </>
  )
}

