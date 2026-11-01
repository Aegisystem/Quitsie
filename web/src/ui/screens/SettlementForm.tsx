import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Member, User } from '../../api/client'
import { liveLedger, liveRecords, type LedgerSummary, settlementRecord } from '../../local'
import { Figure, figureState, Icon } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { amountError, balanceError } from '../forms/amountError'
import { CurrencyOptions } from '../forms/CurrencyOptions'
import { engine, ledgerName, myMember, replica, useLive } from '../data'
import { liveUsedCurrencies, useUser } from '../forms/data'
import { activeMembers } from '../forms/people'
import { initialPeople, type SettlementFix, settlementRejectionText } from '../forms/settlement'
import { todayISO } from '../dates'
import { amountPlaceholder, formatAmountInput, formatMoney, readAmount } from '../money'
import { navigate } from '../router'

interface Suggestion {
  from?: string
  to?: string
  amount?: string
  currency?: string
}

export function SettlementForm({ ledgerId, fromRejected, ...suggestion }: Suggestion & { ledgerId: string; fromRejected?: string }) {
  const { t } = useTranslation()
  const user = useUser()
  const [summary, loaded] = useLive(`ledger:${ledgerId}`, () => liveLedger(replica, ledgerId))
  const [records, recordsLoaded] = useLive('records', () => liveRecords(replica))
  const back = { name: 'home' as const }
  if (!loaded || !recordsLoaded) return <Screen back={back} title={t('settlement.title')} children={null} />
  const me = user && myMember(summary?.members ?? [], user.id)
  if (!summary || !user || !me) {
    return (
      <Screen back={back} title={t('settlement.title')}>
        <p className="empty">{t('ledger.unknown')}</p>
      </Screen>
    )
  }
  const rejected = records?.rejected.find((record) => record.id === fromRejected && record.ledgerId === ledgerId)
  const fix = rejected?.kind === 'settlement' ? { id: rejected.id, reason: rejected.reason, draft: rejected.settlement } : undefined
  return <SettlementFields summary={summary} user={user} me={me} fix={fix} {...suggestion} />
}

function SettlementFields({
  summary,
  user,
  me,
  fix,
  from,
  to,
  amount,
  currency: suggestedCurrency,
}: Suggestion & { summary: LedgerSummary; user: User; me: Member; fix?: SettlementFix }) {
  const { t } = useTranslation()
  const { ledger, members } = summary
  const online = useOnline()
  const ids = useId()
  const [used] = useLive(`usedCurrencies:${user.id}`, () => liveUsedCurrencies(replica, user.id))

  // Only between active Members.
  const active = activeMembers(members, me)
  const activeId = (id?: string) => active.find((m) => m.id === id)?.id
  const groupCurrency = ledger.kind === 'group' ? ledger.currency : undefined
  const draft = fix?.draft
  const [chosenCurrency, setCurrency] = useState(draft?.currency ?? suggestedCurrency ?? user.preferredCurrency)
  const currency = groupCurrency ?? chosenCurrency
  const suggested = amount ? Number(amount) : undefined

  const start = initialPeople(active, me, { from, to, fix })
  const [fromId, setFromId] = useState(start.from)
  const [toId, setToId] = useState(start.to)
  // A record to fix keeps its number even if the Group's currency changed: the
  // same relabeling every other record went through.
  const [amountText, setAmountText] = useState(
    draft ? formatAmountInput(draft.amount, draft.currency) : suggested ? formatAmountInput(suggested, currency) : '',
  )
  const [date, setDate] = useState(draft?.date ?? todayISO())
  const [touched, setTouched] = useState(false)
  const [saving, setSaving] = useState(false)

  const nameOf = (id: string) => (id === me.id ? t('common.you') : (members.find((m) => m.id === id)?.name ?? ''))
  const read = readAmount(amountText, currency)
  const value = read.ok ? read.value : 0
  const sameSuggestedPair = from === fromId && to === toId && suggested !== undefined && (groupCurrency !== undefined || currency === suggestedCurrency)
  const people = fromId === toId ? t('settlement.samePerson') : !activeId(fromId) || !activeId(toId) ? t('settlement.onlyActive') : undefined
  const settlement = settlementRecord({ fromMemberId: fromId, toMemberId: toId, amount: value, currency })
  const errors = {
    people,
    amount:
      amountError(read, currency, t) ??
      (people ? undefined : balanceError(summary.balances, { added: { expenses: [], settlements: [settlement] } }, { me: me.id, nameOf }, t)),
  }
  const valid = !errors.people && !errors.amount

  const save = async () => {
    setTouched(true)
    if (!valid || saving) return
    setSaving(true)
    await engine.addSettlement(
      { ledgerId: ledger.id, fromMemberId: fromId, toMemberId: toId, amount: value, currency, date },
      fix ? { replacing: fix.id } : undefined,
    )
    toast(online ? t('settlement.saved') : t('settlement.savedOffline'))
    navigate({ name: 'ledger', ledgerId: ledger.id, tab: ledger.kind === 'group' ? 'debts' : undefined }, { replace: true })
  }

  const person = (id: string, set: (v: string) => void, label: string, field: string) => (
    <fieldset className="field">
      <legend>{label}</legend>
      <div className="people-pick" role="radiogroup" aria-label={label}>
        {active.map((m) => (
          <label key={m.id} className="person-chip">
            <input type="radio" name={`${ids}-${field}`} checked={id === m.id} onChange={() => set(m.id)} />
            <Figure state={figureState(m)} size={28} />
            <span>{nameOf(m.id)}</span>
          </label>
        ))}
      </div>
    </fieldset>
  )

  return (
    <Screen
      back={{ name: 'ledger', ledgerId: ledger.id }}
      title={fix ? t('settlement.fixTitle') : t('settlement.title')}
      dock={
        <div className="dock-stack">
          {!online && <p className="dock-note">{t('expenseForm.offlineNote')}</p>}
          {!valid && (
            <p className={`dock-note${touched ? ' tone-problem' : ''}`} id={`${ids}-blocking`}>
              {errors.people ?? errors.amount}
            </p>
          )}
          <button
            type="button"
            className="button button-primary button-block"
            onClick={() => void save()}
            aria-disabled={!valid || saving}
            aria-describedby={!valid ? `${ids}-blocking` : undefined}
          >
            <Icon name="check" />
            {t('settlement.save')}
          </button>
        </div>
      }
    >
      <form
        className="form"
        noValidate
        onSubmit={(e) => {
          e.preventDefault()
          void save()
        }}
      >
        <p className="form-context">{ledgerName(summary, user.id)}</p>

        {fix && <p className="notice notice-problem">{settlementRejectionText(fix, groupCurrency, t)}</p>}

        <div className="settle-pair" aria-hidden="true">
          <span className="settle-person">
            <Figure size={44} />
            <span>{nameOf(fromId)}</span>
          </span>
          <span className="settle-flow">
            <span className="settle-amount">{value > 0 ? formatMoney(value, currency, { trim: false }) : '—'}</span>
            <Icon name="arrow" size={28} />
          </span>
          <span className="settle-person">
            <Figure size={44} />
            <span>{nameOf(toId)}</span>
          </span>
        </div>

        {person(fromId, setFromId, t('settlement.from'), 'from')}
        {person(toId, setToId, t('settlement.to'), 'to')}
        {touched && errors.people && <p className="field-error">{errors.people}</p>}

        <div className="field field-amount">
          <label htmlFor={`${ids}-amount`}>{t('settlement.amount')}</label>
          <div className="amount-input">
            <input
              id={`${ids}-amount`}
              inputMode="decimal"
              autoComplete="off"
              placeholder={amountPlaceholder(currency)}
              value={amountText}
              onChange={(e) => setAmountText(e.target.value)}
              aria-invalid={touched && !!errors.amount}
            />
            {groupCurrency ? (
              <span className="currency-code">{currency}</span>
            ) : (
              <select className="currency-select" aria-label={t('settlement.currency')} value={currency} onChange={(e) => setCurrency(e.target.value)}>
                <CurrencyOptions value={currency} preferred={user.preferredCurrency} used={used} />
              </select>
            )}
          </div>
          {touched && errors.amount && <p className="field-error">{errors.amount}</p>}
          {sameSuggestedPair && suggested !== undefined && (
            <p className="hint">
              {t('settlement.suggested', { amount: formatMoney(suggested, currency, { trim: false }) })}.{' '}
              {value > 0 && value < suggested ? t('settlement.partial', { amount: formatMoney(suggested - value, currency, { trim: false }) }) : value === suggested ? t('settlement.full') : ''}
            </p>
          )}
        </div>

        <div className="field">
          <label htmlFor={`${ids}-date`}>{t('settlement.date')}</label>
          <input id={`${ids}-date`} type="date" className="input input-date" value={date} max={todayISO()} onChange={(e) => setDate(e.target.value || todayISO())} />
        </div>

        <p className="notice">{t('settlement.note')}</p>
        <button type="submit" hidden />
      </form>
    </Screen>
  )
}
