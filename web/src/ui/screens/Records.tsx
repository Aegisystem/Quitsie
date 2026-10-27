import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Amount, SyncMark } from '../components/Balance'
import { CategoryGlyph, Icon, SettlementGlyph } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { demo, ledgerById, ledgerName, memberById, useDemo } from '../demo/store'
import type { RejectedRecord, UnsyncedRecord } from '../demo/types'
import { formatRelative } from '../dates'
import { href } from '../router'

// The tray of UnsyncedRecords and RejectedRecords. A rejected one never
// disappears on its own: the User fixes it or discards it (ADR 0001).
export function Records() {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()

  const unsynced: UnsyncedRecord[] = [
    ...s.expenses.filter((e) => e.sync === 'unsynced').map((expense) => ({ type: 'expense' as const, expense })),
    ...s.settlements.filter((x) => x.sync === 'unsynced').map((settlement) => ({ type: 'settlement' as const, settlement })),
  ]

  return (
    <Screen back={{ name: 'home' }} title={t('records.title')}>
      <section className="section" aria-labelledby="rejected-title">
        <h2 id="rejected-title" className="section-title">
          <span className="sync-mark sync-rejected" aria-hidden="true" />
          {t('records.rejectedTitle')}
          <span className="count">{s.rejected.length}</span>
        </h2>
        {s.rejected.length > 0 ? (
          <>
            <p className="hint">{t('records.rejectedHint')}</p>
            <ul className="rows">
              {s.rejected.map((r) => (
                <RejectedRow key={r.id} rejected={r} />
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
                <li key={r.type === 'expense' ? r.expense.id : r.settlement.id}>
                  <RecordRow record={r} state="unsynced" />
                </li>
              ))}
            </ul>
            <button type="button" className="button button-secondary" disabled={!online || s.syncing} onClick={() => demo.sync()}>
              <Icon name="refresh" size={20} />
              {s.syncing ? t('connection.syncing') : online ? t('records.sendNow') : t('records.waiting')}
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

function RecordRow({ record, state }: { record: UnsyncedRecord; state: 'unsynced' | 'rejected' }) {
  const { t } = useTranslation()
  const s = useDemo()
  const ledgerId = record.type === 'expense' ? record.expense.ledgerId : record.settlement.ledgerId
  const ledger = ledgerById(s, ledgerId)
  const created = record.type === 'expense' ? record.expense.createdAt : record.settlement.createdAt
  const title =
    record.type === 'expense'
      ? record.expense.description
      : `${memberById(s, record.settlement.fromMemberId)?.name ?? ''} → ${memberById(s, record.settlement.toMemberId)?.name ?? ''}`

  return (
    <div className="row">
      <span className="row-glyph">{record.type === 'expense' ? <CategoryGlyph category={record.expense.category} /> : <SettlementGlyph />}</span>
      <span className="row-main">
        <span className="row-title">{title}</span>
        <span className="row-sub">
          {record.type === 'expense' ? t('records.expense') : t('records.settlement')}
          {ledger && ` · ${ledgerName(s, ledger)}`} · {t('records.created', { when: formatRelative(created) })}
        </span>
      </span>
      <span className="row-end">
        {record.type === 'expense' ? (
          <Amount minor={record.expense.total} currency={record.expense.currency} />
        ) : (
          <Amount minor={record.settlement.amount} currency={record.settlement.currency} />
        )}
      </span>
      <SyncMark state={state} />
    </div>
  )
}

function RejectedRow({ rejected }: { rejected: RejectedRecord }) {
  const { t } = useTranslation()
  const [confirming, setConfirming] = useState(false)
  const reason =
    rejected.reason.code === 'currencyChanged'
      ? t('records.reasons.currencyChanged', { from: rejected.reason.from, to: rejected.reason.to })
      : t('records.reasons.notMember')
  const name = rejected.record.type === 'expense' ? rejected.record.expense.description : t('records.settlement')
  const ledgerId = rejected.record.type === 'expense' ? rejected.record.expense.ledgerId : rejected.record.settlement.ledgerId

  return (
    <li className="rejected">
      <RecordRow record={rejected.record} state="rejected" />
      <p className="rejected-reason">{reason}</p>
      {confirming ? (
        <div className="confirm" role="group" aria-label={t('records.discardConfirm', { name })}>
          <p>{t('records.discardConfirm', { name })}</p>
          <div className="button-row">
            <button
              type="button"
              className="button button-danger button-small"
              onClick={() => {
                demo.discardRejected(rejected.id)
                toast(t('records.discarded'))
              }}
            >
              {t('records.discardYes')}
            </button>
            <button type="button" className="button button-quiet button-small" onClick={() => setConfirming(false)}>
              {t('common.cancel')}
            </button>
          </div>
        </div>
      ) : (
        <div className="button-row">
          {rejected.record.type === 'expense' && (
            <a className="button button-secondary button-small" href={href({ name: 'newExpense', ledgerId, fromRejected: rejected.id })}>
              {t('records.fix')}
            </a>
          )}
          <button type="button" className="button button-quiet button-small" onClick={() => setConfirming(true)}>
            {t('records.discard')}
          </button>
        </div>
      )}
    </li>
  )
}
