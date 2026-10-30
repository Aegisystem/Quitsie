import { useId, useState, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { CategoryGlyph, Figure, GroupGlyph, Icon } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { previewOwedShares, type SplitParticipant } from '../split'
import { demo, ledgerById, ledgerMembers, ledgerName, memberById, myLedgers, myMemberId, useDemo } from '../demo/store'
import { categories, splitMethods, type Category, type Expense, type Ledger, type SplitMethod } from '../demo/types'
import { todayISO } from '../dates'
import { minorUnitDigits, offeredCurrencies } from '../../domain'
import { amountPlaceholder, formatAmountInput, formatMoney, formatPercent, formatPercentInput, parseAmount, parsePercent, readAmount } from '../money'
import { href, navigate } from '../router'

export function NewExpense({ ledgerId, fromRejected }: { ledgerId?: string; fromRejected?: string }) {
  const s = useDemo()
  const ledger = ledgerId ? ledgerById(s, ledgerId) : undefined
  if (!ledger || !myMemberId(s, ledger.id)) return <PickLedger />
  const rejected = fromRejected ? s.rejected.find((r) => r.id === fromRejected) : undefined
  const draft = rejected?.record.type === 'expense' ? rejected.record.expense : undefined
  // Re-mount when the ledger changes so the form starts clean.
  return <ExpenseForm key={ledger.id} ledger={ledger} draft={draft} rejectedId={rejected?.id} />
}

function PickLedger() {
  const { t } = useTranslation()
  const s = useDemo()
  return (
    <Screen back={{ name: 'home' }} title={t('expenseForm.title')}>
      <h2 className="page-title">{t('expenseForm.pickLedger')}</h2>
      <p className="hint">{t('expenseForm.pickLedgerHint')}</p>
      <ul className="rows">
        {myLedgers(s).map((l) => {
          const friend = l.kind === 'friendship' ? memberById(s, l.friendMemberId) : undefined
          return (
            <li key={l.id}>
              <a className="row row-link" href={href({ name: 'newExpense', ledgerId: l.id })}>
                <span className="row-glyph">{l.kind === 'group' ? <GroupGlyph /> : <Figure state={friend?.userId ? 'linked' : 'pending'} />}</span>
                <span className="row-main">
                  <span className="row-title">{ledgerName(s, l)}</span>
                  <span className="row-sub">{l.kind === 'group' ? l.currency : t('home.friends')}</span>
                </span>
                <span className="row-end">
                  <Icon name="chevron" size={20} />
                </span>
              </a>
            </li>
          )
        })}
      </ul>
    </Screen>
  )
}

type Values = Record<SplitMethod, Record<string, string>>

function ExpenseForm({ ledger, draft, rejectedId }: { ledger: Ledger; draft?: Expense; rejectedId?: string }) {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const ids = useId()
  const me = myMemberId(s, ledger.id) ?? ''
  const members = ledgerMembers(s, ledger.id).filter((m) => m.status === 'active')
  const nameOf = (id: string) => (id === me ? t('common.you') : (memberById(s, id)?.name ?? ''))

  const groupCurrency = ledger.kind === 'group' ? ledger.currency : undefined
  const [currency, setCurrency] = useState(groupCurrency ?? draft?.currency ?? s.me.preferredCurrency)
  const [amountText, setAmountText] = useState(draft ? formatAmountInput(draft.total, currency) : '')
  const [description, setDescription] = useState(draft?.description ?? '')
  const [category, setCategory] = useState<Category | undefined>(draft?.category)
  const [date, setDate] = useState(draft?.date ?? todayISO())
  const [manyPayers, setManyPayers] = useState((draft?.paidShares.length ?? 1) > 1)
  const [payer, setPayer] = useState(draft?.paidShares[0]?.memberId ?? me)
  const [paidText, setPaidText] = useState<Record<string, string>>({})
  const [method, setMethod] = useState<SplitMethod>(draft?.splitMethod ?? 'equal')
  const [included, setIncluded] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(members.map((m) => [m.id, draft ? draft.owedShares.some((o) => o.memberId === m.id) : true])),
  )
  const [values, setValues] = useState<Values>(() => {
    const initial: Values = {
      equal: {},
      exact: {},
      percentage: {},
      weights: Object.fromEntries(members.map((m) => [m.id, '1'])),
      adjustment: {},
    }
    // Re-editing keeps each Participant's value in the split method.
    for (const o of draft?.owedShares ?? []) {
      if (draft?.splitMethod === 'exact') initial.exact[o.memberId] = formatAmountInput(o.amount, draft.currency)
      if (draft?.splitMethod === 'percentage' && o.splitValue !== undefined) initial.percentage[o.memberId] = formatPercentInput(o.splitValue)
      if (draft?.splitMethod === 'weights' && o.splitValue !== undefined) initial.weights[o.memberId] = String(o.splitValue)
      if (draft?.splitMethod === 'adjustment' && o.splitValue) initial.adjustment[o.memberId] = formatAmountInput(o.splitValue, draft.currency)
    }
    return initial
  })
  const [touched, setTouched] = useState(false)

  const read = readAmount(amountText, currency)
  const total = read.ok ? read.value : 0
  const amountError = (): string | undefined => {
    if (read.ok) return read.value > 0 ? undefined : t('expenseForm.amountRequired')
    switch (read.error) {
      case 'empty':
        return t('expenseForm.amountRequired')
      case 'invalid':
        return t('expenseForm.amountInvalid', { example: formatAmountInput(123450, currency) })
      case 'too-many-decimals':
        return t('expenseForm.amountDecimals', { count: minorUnitDigits(currency), currency })
      case 'too-large':
        return t('expenseForm.amountTooLarge')
    }
  }
  // A form always shows every decimal of the currency.
  const money = (minor: number) => formatMoney(minor, currency, { trim: false })
  const rejected = rejectedId ? s.rejected.find((r) => r.id === rejectedId) : undefined

  const participants: SplitParticipant[] = members.map((m) => {
    const text = values[method][m.id] ?? ''
    switch (method) {
      case 'equal':
        return { memberId: m.id, included: included[m.id] ?? false, value: 0 }
      case 'exact': {
        const v = parseAmount(text, currency) ?? 0
        return { memberId: m.id, included: v !== 0, value: v }
      }
      case 'percentage': {
        const v = parsePercent(text) ?? 0
        return { memberId: m.id, included: v !== 0, value: v }
      }
      case 'weights': {
        const v = Number.parseInt(text, 10)
        return { memberId: m.id, included: v > 0, value: Number.isFinite(v) ? v : 0 }
      }
      case 'adjustment':
        return { memberId: m.id, included: included[m.id] ?? false, value: parseAmount(text, currency) ?? 0 }
    }
  })

  const preview = previewOwedShares(total, method, participants)
  const shareOf = (id: string) => preview.shares.find((x) => x.memberId === id)

  const paidShares = manyPayers
    ? members.map((m) => ({ memberId: m.id, amount: parseAmount(paidText[m.id] ?? '', currency) ?? 0 })).filter((p) => p.amount > 0)
    : [{ memberId: payer, amount: total }]
  const paidLeft = total - paidShares.reduce((sum, p) => sum + p.amount, 0)

  const percentSum = participants.reduce((sum, p) => sum + (method === 'percentage' ? p.value : 0), 0)
  const errors = {
    amount: amountError(),
    description: description.trim() === '' ? t('expenseForm.descriptionRequired') : undefined,
  }
  const splitOk = !preview.issue && preview.unassigned === 0
  const valid = !errors.amount && !errors.description && splitOk && paidLeft === 0

  const setValue = (id: string, text: string) => setValues((v) => ({ ...v, [method]: { ...v[method], [id]: text } }))

  const save = () => {
    setTouched(true)
    if (!valid) return
    demo.addExpense(
      {
        ledgerId: ledger.id,
        description: description.trim(),
        date,
        currency,
        total,
        category,
        splitMethod: method,
        paidShares,
        // The Cuotas as the domain computed them, including those that round
        // down to zero, so that re-editing keeps the split.
        owedShares: (preview.owedShares ?? []).map((o) => ({
          memberId: o.participantId,
          amount: o.amount,
          splitValue: o.splitValue ?? undefined,
        })),
        sync: online ? 'synced' : 'unsynced',
      },
      rejectedId,
    )
    toast(online ? t('expenseForm.saved') : t('expenseForm.savedOffline'))
    navigate({ name: 'ledger', ledgerId: ledger.id }, { replace: true })
  }

  // How much of the total the Cuotas already cover, as ten blocks.
  const progress = method === 'percentage' ? percentSum / 10000 : total > 0 ? preview.assigned / total : 0
  const remainderShares = preview.shares.filter((x) => x.remainder > 0)
  const remainderTotal = remainderShares.reduce((sum, x) => sum + x.remainder, 0)

  let closing: { tone: 'even' | 'problem' | 'quiet'; text: string }
  if (total <= 0) closing = { tone: 'quiet', text: errors.amount ?? t('expenseForm.amountRequired') }
  else if (preview.issue === 'noParticipants') closing = { tone: 'problem', text: t('expenseForm.noParticipants') }
  else if (preview.issue === 'noWeights') closing = { tone: 'problem', text: t('expenseForm.noWeights') }
  else if (preview.issue === 'negativeShare') closing = { tone: 'problem', text: t('expenseForm.negativeShare') }
  else if (preview.issue === 'invalidValue') closing = { tone: 'problem', text: t('expenseForm.invalidValue') }
  else if (preview.issue === 'percentageNot100')
    closing =
      percentSum < 10000
        ? { tone: 'problem', text: t('expenseForm.percentMissing', { value: formatPercent(10000 - percentSum) }) }
        : { tone: 'problem', text: t('expenseForm.percentOver', { value: formatPercent(percentSum - 10000) }) }
  else if (preview.unassigned > 0) closing = { tone: 'problem', text: t('expenseForm.missing', { amount: money(preview.unassigned) }) }
  else if (preview.unassigned < 0) closing = { tone: 'problem', text: t('expenseForm.over', { amount: money(-preview.unassigned) }) }
  else closing = { tone: 'even', text: t('expenseForm.closes') }

  // What stops the save, said next to the button so it never looks dead.
  const blocking =
    errors.amount ??
    errors.description ??
    (!splitOk ? closing.text : undefined) ??
    (paidLeft > 0
      ? t('expenseForm.payersLeft', { amount: money(paidLeft) })
      : paidLeft < 0
        ? t('expenseForm.payersOver', { amount: money(-paidLeft) })
        : undefined)

  const backRoute = { name: 'ledger' as const, ledgerId: ledger.id }

  return (
    <Screen
      back={backRoute}
      title={rejected ? t('expenseForm.fixTitle') : t('expenseForm.title')}
      dock={
        <div className="dock-stack">
          {!online && <p className="dock-note">{t('expenseForm.offlineNote')}</p>}
          {blocking && (
            <p className={`dock-note${touched ? ' tone-problem' : ''}`} id={`${ids}-blocking`}>
              {blocking}
            </p>
          )}
          <button
            type="button"
            className="button button-primary button-block"
            onClick={save}
            aria-disabled={!valid}
            aria-describedby={blocking ? `${ids}-blocking` : undefined}
          >
            <Icon name="check" />
            {online ? t('expenseForm.save') : t('expenseForm.saveOffline')}
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
        <p className="form-context">
          {ledger.kind === 'group' ? <GroupGlyph size={20} /> : <Figure size={20} />}
          {ledgerName(s, ledger)}
        </p>

        {rejected && rejected.reason.code === 'currencyChanged' && (
          <p className="notice notice-problem">
            {t('expenseForm.rejectedNote', { reason: t('records.reasons.currencyChanged', { from: rejected.reason.from, to: rejected.reason.to }) })}
          </p>
        )}

        <div className="field field-amount">
          <label htmlFor={`${ids}-amount`}>{t('expenseForm.amount')}</label>
          <div className="amount-input">
            <input
              id={`${ids}-amount`}
              inputMode="decimal"
              autoComplete="off"
              placeholder={amountPlaceholder(currency)}
              value={amountText}
              onChange={(e) => setAmountText(e.target.value)}
              aria-invalid={touched && !!errors.amount}
              aria-describedby={touched && errors.amount ? `${ids}-amount-error` : undefined}
              autoFocus={!draft}
            />
            {groupCurrency ? (
              <span className="currency-code" title={t('ledger.groupCurrency', { currency: groupCurrency })}>
                {currency}
              </span>
            ) : (
              <select className="currency-select" aria-label={t('expenseForm.currency')} value={currency} onChange={(e) => setCurrency(e.target.value)}>
                {offeredCurrencies().map((c) => (
                  <option key={c}>{c}</option>
                ))}
              </select>
            )}
          </div>
          {touched && errors.amount && (
            <p className="field-error" id={`${ids}-amount-error`}>
              {errors.amount}
            </p>
          )}
        </div>

        <div className="field">
          <label htmlFor={`${ids}-desc`}>{t('expenseForm.description')}</label>
          <input
            id={`${ids}-desc`}
            className="input"
            placeholder={t('expenseForm.descriptionPlaceholder')}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            aria-invalid={touched && !!errors.description}
            aria-describedby={touched && errors.description ? `${ids}-desc-error` : undefined}
            maxLength={120}
          />
          {touched && errors.description && (
            <p className="field-error" id={`${ids}-desc-error`}>
              {errors.description}
            </p>
          )}
        </div>

        <fieldset className="field">
          <legend>{t('categories.label')}</legend>
          <div className="chips chips-scroll" role="radiogroup" aria-label={t('categories.label')}>
            {categories.map((c) => (
              <label key={c} className="chip chip-glyph">
                <input type="radio" name={`${ids}-category`} checked={category === c} onChange={() => setCategory(c)} onClick={() => category === c && setCategory(undefined)} />
                <CategoryGlyph category={c} size={20} />
                <span>{t(`categories.${c}`)}</span>
              </label>
            ))}
          </div>
        </fieldset>

        <div className="field">
          <label htmlFor={`${ids}-date`}>{t('expenseForm.date')}</label>
          <input id={`${ids}-date`} type="date" className="input input-date" value={date} max={todayISO()} onChange={(e) => setDate(e.target.value || todayISO())} />
        </div>

        <fieldset className="field">
          <legend>{t('expenseForm.paidBy')}</legend>
          {!manyPayers ? (
            <div className="payer-row">
              <select className="input" value={payer} onChange={(e) => setPayer(e.target.value)} aria-label={t('expenseForm.paidBy')}>
                {members.map((m) => (
                  <option key={m.id} value={m.id}>
                    {nameOf(m.id)}
                  </option>
                ))}
              </select>
              <button type="button" className="button button-small button-quiet" onClick={() => setManyPayers(true)}>
                {t('expenseForm.severalPayers')}
              </button>
            </div>
          ) : (
            <>
              <ul className="people">
                {members.map((m) => (
                  <li key={m.id} className="p-row p-row-input">
                    <Figure state={m.userId ? 'linked' : 'pending'} size={24} />
                    <span className="p-name">{nameOf(m.id)}</span>
                    <input
                      className="input input-amount"
                      inputMode="decimal"
                      placeholder="0"
                      aria-label={t('expenseForm.valueFor', { name: nameOf(m.id) })}
                      value={paidText[m.id] ?? ''}
                      onChange={(e) => setPaidText((p) => ({ ...p, [m.id]: e.target.value }))}
                    />
                  </li>
                ))}
              </ul>
              <div className="split-status">
                <p className={`closing tone-${paidLeft === 0 ? 'even' : 'problem'}`}>
                  {paidLeft === 0 && <Icon name="check" size={18} />}
                  {paidLeft === 0
                    ? t('expenseForm.payersOk')
                    : paidLeft > 0
                      ? t('expenseForm.payersLeft', { amount: money(paidLeft) })
                      : t('expenseForm.payersOver', { amount: money(-paidLeft) })}
                </p>
                <button type="button" className="button button-small button-quiet" onClick={() => setManyPayers(false)}>
                  {t('expenseForm.onePayer')}
                </button>
              </div>
            </>
          )}
        </fieldset>

        <fieldset className="field">
          <legend>{t('expenseForm.split')}</legend>
          <div className="chips" role="radiogroup" aria-label={t('expenseForm.splitMethod')}>
            {splitMethods.map((m) => (
              <label key={m} className="chip">
                <input type="radio" name={`${ids}-method`} checked={method === m} onChange={() => setMethod(m)} />
                <span>{t(`splitMethods.${m}`)}</span>
              </label>
            ))}
          </div>
          <p className="hint">{t(`splitHints.${method}`)}</p>

          <ul className={`people participants participants-${method}`} aria-label={t('expenseForm.participants')}>
            {members.map((m) => {
              const share = shareOf(m.id)
              const checkable = method === 'equal' || method === 'adjustment'
              const on = participants.find((p) => p.memberId === m.id)?.included ?? false
              // Once the split closes, a Cuota of zero is real (1 cent among three).
              const owes = !!share && on && (share.amount !== 0 || !!preview.owedShares)
              return (
                <li key={m.id} className={`p-row${on ? '' : ' is-out'}`}>
                  {checkable ? (
                    <label className="p-check">
                      <input
                        type="checkbox"
                        checked={included[m.id] ?? false}
                        onChange={(e) => setIncluded((x) => ({ ...x, [m.id]: e.target.checked }))}
                        aria-label={t('expenseForm.includes', { name: nameOf(m.id) })}
                      />
                      <Figure state={m.userId ? 'linked' : 'pending'} size={24} />
                    </label>
                  ) : (
                    <Figure state={m.userId ? 'linked' : 'pending'} size={24} />
                  )}
                  <span className="p-name">{nameOf(m.id)}</span>
                  {method !== 'equal' && (
                    <span className="p-value">
                      <input
                        className="input input-amount"
                        inputMode={method === 'weights' ? 'numeric' : 'decimal'}
                        placeholder={method === 'adjustment' ? '+0' : '0'}
                        aria-label={t('expenseForm.valueFor', { name: nameOf(m.id) })}
                        value={values[method][m.id] ?? ''}
                        disabled={method === 'adjustment' && !included[m.id]}
                        onChange={(e) => setValue(m.id, e.target.value)}
                      />
                      <span className="p-unit" aria-hidden="true">
                        {method === 'percentage' ? '%' : method === 'weights' ? t('expenseForm.weightUnit') : ''}
                      </span>
                    </span>
                  )}
                  <span className="p-share" aria-label={t('expenseForm.shareFor', { name: nameOf(m.id) })}>
                    {owes ? money(share.amount) : '—'}
                    {share && share.remainder > 0 && <sup className="p-remainder">+{money(share.remainder)}</sup>}
                  </span>
                </li>
              )
            })}
          </ul>

          <div className="meter" aria-hidden="true" style={{ '--p': Math.max(0, Math.min(1, progress)) } as CSSProperties}>
            {Array.from({ length: 10 }, (_, i) => (
              <i key={i} className={progress > 1 ? 'is-over' : ''} style={{ '--b': Math.max(0, Math.min(1, progress * 10 - i)) } as CSSProperties} />
            ))}
          </div>
          <p className={`closing tone-${closing.tone}`} role="status">
            {closing.tone === 'even' && <Icon name="check" size={18} />}
            {closing.text}
            {total > 0 && !preview.issue && (
              <span className="closing-sub">{t('expenseForm.assigned', { assigned: money(preview.assigned), total: money(total) })}</span>
            )}
          </p>
          {remainderTotal > 0 && !preview.issue && (
            <p className="hint">
              {t('expenseForm.remainder', { count: remainderShares.length, amount: money(remainderTotal) })}
            </p>
          )}
        </fieldset>
        <button type="submit" hidden />
      </form>
    </Screen>
  )
}
