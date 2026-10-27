import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Figure, Icon } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { demo, ledgerById, ledgerMembers, ledgerName, memberById, myMemberId, useDemo } from '../demo/store'
import { todayISO } from '../dates'
import { formatAmountInput, formatMoney, parseAmount } from '../money'
import { navigate } from '../router'

export function SettlementForm({
  ledgerId,
  from,
  to,
  amount,
  currency: currencyParam,
}: {
  ledgerId: string
  from?: string
  to?: string
  amount?: string
  currency?: string
}) {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const ids = useId()
  const ledger = ledgerById(s, ledgerId)
  const me = ledger ? (myMemberId(s, ledger.id) ?? '') : ''
  const members = ledger ? ledgerMembers(s, ledger.id).filter((m) => m.status === 'active') : []
  const currency = ledger?.kind === 'group' ? ledger.currency : (currencyParam ?? s.me.preferredCurrency)
  const suggested = amount ? Number(amount) : undefined

  const [fromId, setFromId] = useState(from ?? me)
  const [toId, setToId] = useState(to ?? members.find((m) => m.id !== (from ?? me))?.id ?? '')
  const [amountText, setAmountText] = useState(suggested ? formatAmountInput(suggested, currency) : '')
  const [date, setDate] = useState(todayISO())
  const [touched, setTouched] = useState(false)

  if (!ledger) {
    return (
      <Screen back={{ name: 'home' }} title={t('settlement.title')}>
        <p className="empty">{t('ledger.unknown')}</p>
      </Screen>
    )
  }

  const nameOf = (id: string) => (id === me ? t('common.you') : (memberById(s, id)?.name ?? ''))
  const value = parseAmount(amountText, currency) ?? 0
  const sameSuggestedPair = from === fromId && to === toId && suggested !== undefined
  const errors = {
    people: fromId === toId ? t('settlement.samePerson') : undefined,
    amount: value <= 0 ? t('expenseForm.amountRequired') : undefined,
  }
  const valid = !errors.people && !errors.amount

  const save = () => {
    setTouched(true)
    if (!valid) return
    demo.addSettlement({ ledgerId: ledger.id, fromMemberId: fromId, toMemberId: toId, amount: value, currency, date, sync: online ? 'synced' : 'unsynced' })
    toast(online ? t('settlement.saved') : t('settlement.savedOffline'))
    navigate({ name: 'ledger', ledgerId: ledger.id, tab: ledger.kind === 'group' ? 'debts' : undefined }, { replace: true })
  }

  const person = (id: string, set: (v: string) => void, label: string, field: string) => (
    <fieldset className="field">
      <legend>{label}</legend>
      <div className="people-pick" role="radiogroup" aria-label={label}>
        {members.map((m) => (
          <label key={m.id} className="person-chip">
            <input type="radio" name={`${ids}-${field}`} checked={id === m.id} onChange={() => set(m.id)} />
            <Figure state={m.userId ? 'linked' : 'pending'} size={28} />
            <span>{nameOf(m.id)}</span>
          </label>
        ))}
      </div>
    </fieldset>
  )

  return (
    <Screen
      back={{ name: 'ledger', ledgerId: ledger.id }}
      title={t('settlement.title')}
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
            onClick={save}
            aria-disabled={!valid}
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
          save()
        }}
      >
        <p className="form-context">{ledgerName(s, ledger)}</p>

        <div className="settle-pair" aria-hidden="true">
          <span className="settle-person">
            <Figure size={44} />
            <span>{nameOf(fromId)}</span>
          </span>
          <span className="settle-flow">
            <span className="settle-amount">{value > 0 ? formatMoney(value, currency) : '—'}</span>
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
              placeholder="0"
              value={amountText}
              onChange={(e) => setAmountText(e.target.value)}
              aria-invalid={touched && !!errors.amount}
            />
            <span className="currency-code">{currency}</span>
          </div>
          {touched && errors.amount && <p className="field-error">{errors.amount}</p>}
          {sameSuggestedPair && suggested !== undefined && (
            <p className="hint">
              {t('settlement.suggested', { amount: formatMoney(suggested, currency) })}.{' '}
              {value > 0 && value < suggested ? t('settlement.partial', { amount: formatMoney(suggested - value, currency) }) : value === suggested ? t('settlement.full') : ''}
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
