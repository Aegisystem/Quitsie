import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { overMaxBalance } from '../../domain'
import { liveHome, liveRecords } from '../../local'
import { Amount, SyncMark } from '../components/Balance'
import { CategoryGlyph, Icon, SettlementGlyph } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { engine, ledgerName, relabelAmount, replica, useLive, useSyncStatus } from '../data'
import { formatRelative } from '../dates'
import { href } from '../router'
import type { LedgerSummary, QueuedRecord, RejectedRecord, User } from '../types'

// The tray of UnsyncedRecords and RejectedRecords. A rejected one never
// disappears on its own: the User fixes it or discards it (ADR 0001).
export function Records({ user }: { user: User }) {
  const { t } = useTranslation()
  const online = useOnline()
  const syncing = useSyncStatus() === 'syncing'
  const [records] = useLive('records', () => liveRecords(replica))
  const [home] = useLive('home', () => liveHome(replica))
  const rejected = records?.rejected ?? []
  const unsynced = records?.unsynced ?? []
  // A record can outlive its ledger, when the User lost access to it.
  const summaryOf = (ledgerId: string) => home?.find(({ ledger }) => ledger.id === ledgerId)

  return (
    <Screen back={{ name: 'home' }} title={t('records.title')}>
      <section className="section" aria-labelledby="rejected-title">
        <h2 id="rejected-title" className="section-title">
          <span className="sync-mark sync-rejected" aria-hidden="true" />
          {t('records.rejectedTitle')}
          <span className="count">{rejected.length}</span>
        </h2>
        {rejected.length > 0 ? (
          <>
            <p className="hint">{t('records.rejectedHint')}</p>
            {!online && <p className="hint">{t('records.offlineHint')}</p>}
            <ul className="rows">
              {rejected.map((r) => (
                <RejectedRow key={r.id} rejected={r} summary={summaryOf(r.ledgerId)} user={user} />
              ))}
            </ul>
          </>
        ) : (
          <p className="empty">{t('records.noRejected')}</p>
        )}
      </section>

      <section className="section" aria-labelledby="unsynced-title">
        <h2 id="unsynced-title" className="section-title">
          <span className="sync-mark sync-unsynced" aria-hidden="true" />
          {t('records.unsyncedTitle')}
          <span className="count">{unsynced.length}</span>
        </h2>
        {unsynced.length > 0 ? (
          <>
            <p className="hint">{t('records.unsyncedHint')}</p>
            <ul className="rows">
              {unsynced.map((r) => (
                <li key={r.id}>
                  <RecordRow record={r} state="unsynced" summary={summaryOf(r.ledgerId)} user={user} />
                </li>
              ))}
            </ul>
            <button type="button" className="button button-secondary" disabled={!online || syncing} onClick={() => void engine.sync()}>
              <Icon name="refresh" size={20} />
              {syncing ? t('connection.syncing') : online ? t('records.sendNow') : t('records.waiting')}
            </button>
          </>
        ) : (
          <p className="empty">
            {t('records.empty')} {t('records.emptyWink')}
          </p>
        )}
      </section>
    </Screen>
  )
}

function RecordRow({ record, state, summary, user }: { record: QueuedRecord; state: 'unsynced' | 'rejected'; summary?: LedgerSummary; user: User }) {
  const { t } = useTranslation()
  const nameOf = (memberId: string) => {
    const member = summary?.members.find(({ id }) => id === memberId)
    return member?.userId === user.id ? t('common.you') : member?.name
  }
  const people = record.kind === 'settlement' ? [nameOf(record.settlement.fromMemberId), nameOf(record.settlement.toMemberId)] : []
  const title = record.kind === 'expense' ? record.expense.description : people.every(Boolean) ? people.join(' → ') : t('records.settlement')

  return (
    <div className="row">
      <span className="row-glyph">{record.kind === 'expense' ? <CategoryGlyph category={record.expense.category} /> : <SettlementGlyph />}</span>
      <span className="row-main">
        <span className="row-title">{title}</span>
        <span className="row-sub">
          {record.kind === 'expense' ? t('records.expense') : t('records.settlement')}
          {summary && ` · ${ledgerName(summary, user.id)}`} · {t('records.created', { when: formatRelative(record.queuedAt) })}
        </span>
      </span>
      <span className="row-end">
        {record.kind === 'expense' ? (
          <Amount minor={record.expense.total} currency={record.expense.currency} />
        ) : (
          <Amount minor={record.settlement.amount} currency={record.settlement.currency} />
        )}
      </span>
      <SyncMark state={state} />
    </div>
  )
}

function RejectedRow({ rejected, summary, user }: { rejected: RejectedRecord; summary?: LedgerSummary; user: User }) {
  const { t } = useTranslation()
  const online = useOnline()
  const [confirming, setConfirming] = useState(false)
  const currency = rejected.kind === 'expense' ? rejected.expense.currency : rejected.settlement.currency
  const group = summary?.ledger.kind === 'group' ? summary.ledger : undefined
  const name = rejected.kind === 'expense' ? rejected.expense.description : t('records.settlement')

  const reason =
    rejected.reason !== 'currency_mismatch'
      ? t(`records.reasons.${rejected.reason}`)
      : group
        ? t('records.reasons.currencyChanged', { from: currency, to: group.currency })
        : t('records.reasons.currencyChangedTo')

  // The form corrects an Expense. A Settlement only has its currency to confirm,
  // when the Group's changed: the same number, relabeled like every other record.
  const canFixExpense = rejected.kind === 'expense' && summary && rejected.reason !== 'ledger_unavailable'
  const relabeled =
    rejected.kind === 'settlement' && rejected.reason === 'currency_mismatch' && group ? relabelAmount(rejected.settlement.amount, currency, group.currency) : undefined
  // In a currency with more decimals the same number is a larger amount, which
  // may not fit in someone's Balance.
  const pastLimit =
    rejected.kind === 'settlement' &&
    relabeled !== undefined &&
    !!group &&
    overMaxBalance(summary?.balances ?? [], {
      expenses: [],
      settlements: [{ id: rejected.id, from: rejected.settlement.fromMemberId, to: rejected.settlement.toMemberId, amount: relabeled, currency: group.currency }],
    }) !== undefined

  const fixCurrency = async () => {
    if (rejected.kind !== 'settlement' || !group || relabeled === undefined || pastLimit) return
    await engine.addSettlement({ ...rejected.settlement, amount: relabeled, currency: group.currency }, { replacing: rejected.id })
    toast(t('records.fixedCurrency'))
  }

  const discard = async () => {
    await engine.discardRecord(rejected.id)
    toast(t('records.discarded'))
  }

  return (
    <li className="rejected">
      <RecordRow record={rejected} state="rejected" summary={summary} user={user} />
      <p className="rejected-reason">{reason}</p>
      {pastLimit && group && <p className="hint">{t('records.fixCurrencyBalance', { currency: group.currency })}</p>}
      {confirming ? (
        <div className="confirm" role="group" aria-label={t('records.discardConfirm', { name })}>
          <p>{t('records.discardConfirm', { name })}</p>
          <div className="button-row">
            <button type="button" className="button button-danger button-small" disabled={!online} onClick={() => void discard()}>
              {t('records.discardYes')}
            </button>
            <button type="button" className="button button-quiet button-small" onClick={() => setConfirming(false)}>
              {t('common.cancel')}
            </button>
          </div>
        </div>
      ) : (
        <div className="button-row">
          {canFixExpense && (
            <a
              className="button button-secondary button-small"
              href={online ? href({ name: 'newExpense', ledgerId: rejected.ledgerId, fromRejected: rejected.id }) : undefined}
              aria-disabled={!online}
              title={online ? undefined : t('common.onlineOnly')}
            >
              {t('records.fix')}
            </a>
          )}
          {relabeled !== undefined && group && !pastLimit && (
            <button type="button" className="button button-secondary button-small" disabled={!online} title={online ? undefined : t('common.onlineOnly')} onClick={() => void fixCurrency()}>
              {t('records.fixCurrency', { currency: group.currency })}
            </button>
          )}
          <button type="button" className="button button-quiet button-small" disabled={!online} title={online ? undefined : t('common.onlineOnly')} onClick={() => setConfirming(true)}>
            {t('records.discard')}
          </button>
        </div>
      )}
    </li>
  )
}
