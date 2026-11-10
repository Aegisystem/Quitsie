import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { liveBalances, liveDebts, liveExpenses, liveLedger, liveRecords, liveSettlements } from '../../local'
import { Amount, BalanceBands, SyncMark, Tally, TallyLegend, toneOf } from '../components/Balance'
import { CategoryGlyph, Figure, figureState, Icon, SettlementGlyph } from '../components/icons'
import { FirstSync, Screen, toast, useOnline } from '../components/Shell'
import { actions, balanceProof, friendOf, ledgerName, myBalance, myMember, replica, useLive, useReplicaFilled, type RecordKind } from '../data'
import { formatDay } from '../dates'
import { formatMoney, tallyUnit } from '../money'
import { href, type Route } from '../router'
import type { Debt, ExpenseView, Group, Ledger as LedgerT, Member, SettlementView, SyncState, User } from '../types'
import { ActivityList } from './Activity'

type Tab = 'expenses' | 'balances' | 'debts' | 'activity'

// That screen is built apart, so its address is written out here.
const settingsHref = (ledgerId: string) => `#/ledgers/${encodeURIComponent(ledgerId)}/settings`

export function Ledger({ ledgerId, tab, user }: { ledgerId: string; tab?: string; user: User }) {
  const { t } = useTranslation()
  const online = useOnline()
  const [summary, loaded] = useLive(`ledger:${ledgerId}`, () => liveLedger(replica, ledgerId))
  const [debts = []] = useLive(`debts:${ledgerId}`, () => liveDebts(replica, ledgerId))
  const [records] = useLive('records', () => liveRecords(replica))
  const filled = useReplicaFilled()
  const me = summary && myMember(summary.members, user.id)

  if (!summary || !me) {
    return (
      <Screen back={{ name: 'home' }} title={t('nav.home')}>
        {filled === false ? <FirstSync /> : loaded && <p className="empty">{t('ledger.unknown')}</p>}
      </Screen>
    )
  }

  const { ledger, members } = summary
  const tabs: Tab[] = ledger.kind === 'group' ? ['expenses', 'balances', 'debts', 'activity'] : ['expenses', 'activity']
  const current: Tab = tabs.includes(tab as Tab) ? (tab as Tab) : 'expenses'
  const mine = myBalance(summary, me.id, user.preferredCurrency)
  const name = ledgerName(summary, user.id)
  const friend = ledger.kind === 'friendship' ? friendOf(members, user.id) : undefined
  const canInvite = ledger.kind === 'group' || (friend && !friend.userId)
  const settleRoute = firstSettlement(ledger, me.id, friend, mine, debts)

  return (
    <Screen
      back={{ name: 'home' }}
      title={name}
      actions={
        <>
          {canInvite && (
            <a
              className={`icon-button${online ? '' : ' is-disabled'}`}
              href={online ? href({ name: 'inviteLink', ledgerId: ledger.id }) : undefined}
              aria-disabled={!online}
              aria-label={t('ledger.invite')}
              title={online ? t('ledger.invite') : t('common.onlineOnly')}
            >
              <Icon name="share" />
            </a>
          )}
          <a className="icon-button" href={settingsHref(ledger.id)} aria-label={t('ledger.settings')} title={t('ledger.settings')}>
            <Icon name="settings" />
          </a>
        </>
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
          provisional={records?.unsynced.some((record) => record.ledgerId === ledger.id)}
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
          ? `${t('ledger.groupCurrency', { currency: ledger.currency })} · ${t('ledger.members', { count: members.filter((m) => m.status === 'active').length })}`
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
        {current === 'expenses' && <ExpensesTab ledger={ledger} members={members} me={me.id} />}
        {current === 'balances' && ledger.kind === 'group' && <BalancesTab group={ledger} members={members} me={me.id} />}
        {current === 'debts' && ledger.kind === 'group' && <DebtsTab group={ledger} members={members} me={me.id} debts={debts} />}
        {current === 'activity' && <ActivityList ledgerId={ledger.id} user={user} />}
      </div>
    </Screen>
  )
}

// Where "Liquidar" starts: the first suggested Debt that involves the User, if any.
function firstSettlement(ledger: LedgerT, me: string, friend: Member | undefined, mine: ReadonlyMap<string, number>, debts: readonly Debt[]): Route {
  const settle = { name: 'settle' as const, ledgerId: ledger.id }
  if (ledger.kind === 'friendship') {
    const [currency, amount] = [...mine][0] ?? []
    if (!currency || !amount || !friend) return settle
    return amount < 0 ? { ...settle, from: me, to: friend.id, amount: String(-amount), currency } : { ...settle, from: friend.id, to: me, amount: String(amount), currency }
  }
  const debt = debts.find((d) => d.fromMemberId === me || d.toMemberId === me)
  return debt ? { ...settle, from: debt.fromMemberId, to: debt.toMemberId, amount: String(debt.amount) } : settle
}

function useNames(members: readonly Member[], me: string) {
  const { t } = useTranslation()
  return (memberId: string) => (memberId === me ? t('common.you') : (members.find((m) => m.id === memberId)?.name ?? '?'))
}

type Item = { kind: 'expense'; date: string; at: string; sync: SyncState; expense: ExpenseView } | { kind: 'settlement'; date: string; at: string; sync: SyncState; settlement: SettlementView }

function ExpensesTab({ ledger, members, me }: { ledger: LedgerT; members: readonly Member[]; me: string }) {
  const { t } = useTranslation()
  const online = useOnline()
  const name = useNames(members, me)
  const [expenses] = useLive(`expenses:${ledger.id}`, () => liveExpenses(replica, ledger.id))
  const [settlements] = useLive(`settlements:${ledger.id}`, () => liveSettlements(replica, ledger.id))
  if (!expenses || !settlements) return null

  const items: Item[] = [
    ...expenses.map((expense) => ({ kind: 'expense' as const, date: expense.date, at: expense.createdAt, sync: expense.sync, expense })),
    ...settlements.map((settlement) => ({ kind: 'settlement' as const, date: settlement.date, at: settlement.createdAt, sync: settlement.sync, settlement })),
  ].sort((a, b) => b.date.localeCompare(a.date) || b.at.localeCompare(a.at))

  if (items.length === 0) {
    const alone = ledger.kind === 'group' && members.filter((m) => m.status === 'active').length === 1
    return (
      <>
        <p className="empty">{t('ledger.emptyExpenses')}</p>
        {alone && (
          <>
            <p className="hint">{t('ledger.aloneHint')}</p>
            <a className="button button-secondary" href={settingsHref(ledger.id)}>
              {t('ledger.addMembers')}
            </a>
          </>
        )}
      </>
    )
  }

  const days = [...new Set(items.map((i) => i.date))]

  return (
    <div className="day-list">
      {!online && items.some((i) => i.sync === 'synced') && <p className="hint">{t('ledger.recordsOffline')}</p>}
      {days.map((day) => (
        <section key={day} aria-label={formatDay(day)}>
          <h3 className="day-title">{formatDay(day)}</h3>
          <ul className="rows">
            {items
              .filter((i) => i.date === day)
              .map((item) =>
                item.kind === 'expense' ? (
                  <ExpenseRow key={item.expense.id} expense={item.expense} me={me} name={name} />
                ) : (
                  <SettlementRow key={item.settlement.id} settlement={item.settlement} me={me} name={name} />
                ),
              )}
          </ul>
        </section>
      ))}
    </div>
  )
}

// One Expense or Settlement in the list. What it offers depends on where it is:
// on the server it can be deleted or restored, online only (ADR 0001); in the
// queue it can not, and a rejected one leads to the Records tray. Nothing here
// assumes the server said yes: the row only changes when the sync brings it.
function RecordRow({
  kind,
  id,
  sync,
  deleted,
  className = '',
  glyph,
  title,
  sub,
  end,
  editHref,
}: {
  kind: RecordKind
  id: string
  sync: SyncState
  deleted: boolean
  className?: string
  glyph: ReactNode
  title: ReactNode
  sub: ReactNode
  end?: ReactNode
  editHref?: string
}) {
  const { t } = useTranslation()
  const online = useOnline()
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [problem, setProblem] = useState<string>()

  const run = async (action: 'delete' | 'restore') => {
    setBusy(true)
    setProblem(undefined)
    const result = await (action === 'delete' ? actions.deleteRecord(kind, id) : actions.restoreRecord(kind, id))
    setBusy(false)
    if (result === 'done') {
      setOpen(false)
      if (action === 'delete') toast(kind === 'expense' ? t('ledger.expenseDeletedDone') : t('ledger.settlementDeletedDone'))
      else toast(kind === 'expense' ? t('ledger.expenseRestoredDone') : t('ledger.settlementRestoredDone'))
    } else if (result === 'formerMember') {
      setProblem(action === 'delete' ? t('ledger.deleteFormerMember') : t('ledger.restoreFormerMember'))
    } else if (result === 'balance') {
      setProblem(action === 'delete' ? t('ledger.deleteBalance') : t('ledger.restoreBalance'))
    } else {
      setProblem(result === 'gone' ? t('ledger.recordGone') : result === 'offline' ? t('common.noAnswer') : t('common.tryAgain'))
    }
  }

  const content = (
    <>
      <span className="row-glyph">{glyph}</span>
      <span className="row-main">
        <span className="row-title">{deleted ? <s>{title}</s> : title}</span>
        <span className="row-sub">{sub}</span>
      </span>
      <span className="row-end">
        {deleted ? (
          <button
            type="button"
            className="button button-small button-quiet"
            disabled={!online || busy}
            title={online ? undefined : t('common.onlineOnly')}
            onClick={() => void run('restore')}
          >
            <Icon name="restore" size={18} />
            {busy ? t('ledger.restoring') : t('ledger.restore')}
          </button>
        ) : (
          end
        )}
      </span>
      {deleted ? <span className="sync-cell" /> : <SyncMark state={sync} />}
    </>
  )

  if (sync === 'rejected') {
    return (
      <li>
        <a className={`row row-link ${className}`} href={href({ name: 'records' })}>
          {content}
        </a>
      </li>
    )
  }
  if (sync === 'unsynced' || deleted) {
    return (
      <li>
        <div className={`row ${deleted ? 'row-deleted' : ''} ${className}`}>{content}</div>
        {problem && <p className="rejected-reason">{problem}</p>}
      </li>
    )
  }

  const toggle = () => {
    setOpen(!open)
    setProblem(undefined)
  }

  return (
    <li>
      <button type="button" className={`row row-button ${className}`} aria-expanded={open} onClick={toggle}>
        {content}
      </button>
      {open && (
        <div className="row-panel">
          {editHref &&
            (online ? (
              <a className="button button-small button-secondary" href={editHref}>
                {t('ledger.editExpense')}
              </a>
            ) : (
              <button type="button" className="button button-small button-secondary" disabled title={t('common.onlineOnly')}>
                {t('ledger.editExpense')}
              </button>
            ))}
          <button
            type="button"
            className="button button-small button-secondary"
            disabled={!online || busy}
            title={online ? undefined : t('common.onlineOnly')}
            onClick={() => void run('delete')}
          >
            {busy ? t('ledger.deleting') : kind === 'expense' ? t('ledger.deleteExpense') : t('ledger.deleteSettlement')}
          </button>
        </div>
      )}
      {problem && <p className="rejected-reason">{problem}</p>}
    </li>
  )
}

function ExpenseRow({ expense, me, name }: { expense: ExpenseView; me: string; name: (id: string) => string }) {
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

  return (
    <RecordRow
      kind="expense"
      id={expense.id}
      sync={expense.sync}
      deleted={!!expense.deletedAt}
      editHref={href({ name: 'editExpense', ledgerId: expense.ledgerId, expenseId: expense.id })}
      glyph={<CategoryGlyph category={expense.category} />}
      title={expense.description}
      sub={
        expense.sync === 'rejected' ? (
          t('ledger.rejectedRow')
        ) : expense.deletedAt ? (
          t('ledger.deleted')
        ) : (
          <>
            {payer} · <span className={`tone-${share.tone}`}>{share.text}</span>
          </>
        )
      }
      end={<Amount minor={expense.total} currency={expense.currency} />}
    />
  )
}

function SettlementRow({ settlement, me, name }: { settlement: SettlementView; me: string; name: (id: string) => string }) {
  const { t } = useTranslation()
  const { fromMemberId, toMemberId } = settlement
  const amount = formatMoney(settlement.amount, settlement.currency)
  const title =
    fromMemberId === me
      ? t('ledger.settlementLineYouPaid', { amount, to: name(toMemberId) })
      : toMemberId === me
        ? t('ledger.settlementLinePaidYou', { amount, from: name(fromMemberId) })
        : t('ledger.settlementLine', { amount, from: name(fromMemberId), to: name(toMemberId) })

  return (
    <RecordRow
      kind="settlement"
      id={settlement.id}
      sync={settlement.sync}
      deleted={!!settlement.deletedAt}
      className="row-settlement"
      glyph={<SettlementGlyph />}
      title={title}
      sub={settlement.sync === 'rejected' ? t('ledger.rejectedRow') : settlement.deletedAt ? t('ledger.settlementDeleted') : t('ledger.settlementLabel')}
    />
  )
}

function BalancesTab({ group, members, me }: { group: Group; members: readonly Member[]; me: string }) {
  const { t } = useTranslation()
  const name = useNames(members, me)
  const [balances] = useLive(`balances:${group.id}`, () => liveBalances(replica, group.id))
  const [expenses] = useLive(`expenses:${group.id}`, () => liveExpenses(replica, group.id))
  const [settlements] = useLive(`settlements:${group.id}`, () => liveSettlements(replica, group.id))
  if (!balances || !expenses || !settlements) return null

  const shown = [...members].sort((a, b) => (a.id === me ? -1 : b.id === me ? 1 : 0))
  const amounts = shown.map((m) => balances.find((b) => b.participantId === m.id && b.currency === group.currency)?.amount ?? 0)
  const unit = tallyUnit(Math.max(1, ...amounts.map(Math.abs)), group.currency, 9)

  return (
    <>
      <div className="chart-legend">
        <p>{t('ledger.balancesLegend')}</p>
        <TallyLegend unit={unit} currency={group.currency} />
      </div>
      <ul className="chart">
        {shown.map((m, i) => {
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
                    {m.id === me
                      ? tone === 'favor'
                        ? t('balance.youOwedShort')
                        : tone === 'against'
                          ? t('balance.youOweShort')
                          : t('balance.youSettledShort')
                      : tone === 'favor'
                        ? t('balance.memberOwed')
                        : tone === 'against'
                          ? t('balance.memberOwes')
                          : t('balance.memberSettled')}
                  </span>
                  {amount !== 0 && <Amount minor={Math.abs(amount)} currency={group.currency} tone={tone} />}
                </span>
              </div>
              <div className={`diverge tone-${tone}`} aria-hidden="true">
                {/* Both rows stay, empty or not, so that one can be seen emptying. */}
                <span className="diverge-side diverge-left">
                  <Tally amount={Math.min(amount, 0)} unit={unit} align="end" />
                </span>
                <span className="diverge-axis">{amount === 0 && <i className="even-dot" />}</span>
                <span className="diverge-side diverge-right">
                  <Tally amount={Math.max(amount, 0)} unit={unit} />
                </span>
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
      {shown.some((m) => figureState(m) === 'pending') && <p className="hint">{t('ledger.pendingHint')}</p>}
    </>
  )
}

function DebtsTab({ group, members, me, debts }: { group: Group; members: readonly Member[]; me: string; debts: readonly Debt[] }) {
  const { t } = useTranslation()
  const name = useNames(members, me)
  const [records] = useLive('records', () => liveRecords(replica))
  const unit = tallyUnit(Math.max(1, ...debts.map((d) => d.amount)), group.currency, 7)
  const memberById = (id: string) => members.find((m) => m.id === id)

  // Debt simplification is always on (docs/reglas-de-negocio.md): the fewest
  // payments that leave everyone even.
  return (
    <>
      <h2 className="section-title">{t('ledger.debtsTitle')}</h2>
      {records?.unsynced.some((record) => record.ledgerId === group.id) && <p className="hint">{t('ledger.debtsWithoutQueue')}</p>}
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
                <li key={`${d.fromMemberId}-${d.toMemberId}-${d.currency}`} className={`debt${mine ? ' debt-mine' : ''}`}>
                  <div className="debt-people" aria-hidden="true">
                    <Figure state={figureState(memberById(d.fromMemberId))} size={26} />
                    <Icon name="arrow" size={20} />
                    <Figure state={figureState(memberById(d.toMemberId))} size={26} />
                  </div>
                  <div className="debt-main">
                    <p className="debt-label">{label}</p>
                    <div className="debt-figure">
                      <Amount minor={d.amount} currency={d.currency} tone={d.fromMemberId === me ? 'against' : d.toMemberId === me ? 'favor' : undefined} />
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
