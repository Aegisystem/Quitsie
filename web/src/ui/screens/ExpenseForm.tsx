import { useEffect, useId, useState, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type Category, type Member, type NewExpense as NewExpenseRecord, type SplitMethod, type User } from '../../api/client'
import { liveExpenses, liveHome, liveLedger, liveRecords, type ExpenseView, type LedgerSummary, type RejectionReason } from '../../local'
import { CategoryGlyph, Figure, figureState, GroupGlyph, Icon } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { amountError } from '../forms/amountError'
import { CurrencyOptions } from '../forms/CurrencyOptions'
import { engine, ledgerName, myMember, replica, useLive } from '../data'
import { liveUsedCurrencies, useUser } from '../forms/data'
import {
  changedFormerShare,
  formerParticipant,
  formerShares,
  initialPaidText,
  initialSplitValues,
  methodsFor,
  noFormerShares,
  toOwedShares,
  type SplitValues,
} from '../forms/expense'
import { activeMembers, isActive } from '../forms/people'
import { updateExpense, type UpdateOutcome } from '../forms/updateExpense'
import { readValue, valueError } from '../forms/values'
import { categories } from '../types'
import { previewOwedShares, type SplitParticipant } from '../split'
import { formatDay, todayISO } from '../dates'
import { amountPlaceholder, formatAmountInput, formatMoney, formatPercent, readAmount } from '../money'
import { href, navigate } from '../router'

export function NewExpense({ ledgerId, fromRejected }: { ledgerId?: string; fromRejected?: string }) {
  if (!ledgerId) return <PickLedger />
  // Re-mount when the ledger changes so the form starts clean.
  return <NewExpenseIn key={ledgerId} ledgerId={ledgerId} fromRejected={fromRejected} />
}

function NewExpenseIn({ ledgerId, fromRejected }: { ledgerId: string; fromRejected?: string }) {
  const { t } = useTranslation()
  const user = useUser()
  const [summary, ledgerLoaded] = useLive(`ledger:${ledgerId}`, () => liveLedger(replica, ledgerId))
  const [records, recordsLoaded] = useLive('records', () => liveRecords(replica))
  if (!ledgerLoaded || !recordsLoaded) return <Screen back={{ name: 'home' }} title={t('expenseForm.title')} children={null} />
  const me = user && myMember(summary?.members ?? [], user.id)
  if (!summary || !user || !me) return <PickLedger />
  const rejected = records?.rejected.find((record) => record.id === fromRejected)
  const purpose: Purpose =
    rejected?.kind === 'expense' ? { kind: 'fix', id: rejected.id, reason: rejected.reason, draft: rejected.expense } : { kind: 'new' }
  return <ExpenseForm summary={summary} user={user} me={me} purpose={purpose} />
}

function PickLedger() {
  const { t } = useTranslation()
  const user = useUser()
  const [home = []] = useLive('home', () => liveHome(replica))
  const mine = home.flatMap((summary) => {
    const me = user && myMember(summary.members, user.id)
    return user && me ? [{ ...summary, me, name: ledgerName(summary, user.id) }] : []
  })
  const groupsFirst = mine.sort((a, b) => b.ledger.kind.localeCompare(a.ledger.kind) || a.name.localeCompare(b.name))
  return (
    <Screen back={{ name: 'home' }} title={t('expenseForm.title')}>
      <h2 className="page-title">{t('expenseForm.pickLedger')}</h2>
      <p className="hint">{t('expenseForm.pickLedgerHint')}</p>
      <ul className="rows">
        {groupsFirst.map(({ ledger, members, me, name }) => {
          const friend = members.find((member) => member.id !== me.id)
          return (
            <li key={ledger.id}>
              <a className="row row-link" href={href({ name: 'newExpense', ledgerId: ledger.id })}>
                <span className="row-glyph">{ledger.kind === 'group' ? <GroupGlyph /> : <Figure state={friend?.userId ? 'linked' : 'pending'} />}</span>
                <span className="row-main">
                  <span className="row-title">{name}</span>
                  <span className="row-sub">{ledger.kind === 'group' ? ledger.currency : t('home.friends')}</span>
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

// Editing happens online, against the version the form was filled from (ADR 0001).
export function EditExpense({ ledgerId, expenseId }: { ledgerId: string; expenseId: string }) {
  const { t } = useTranslation()
  const user = useUser()
  // A key of its own: the form starts from what the replica has now, never from
  // a value remembered from an earlier visit.
  const visit = useId()
  const [summary, ledgerLoaded] = useLive(`ledger:${ledgerId}`, () => liveLedger(replica, ledgerId))
  const [expenses, expensesLoaded] = useLive(`editExpense:${visit}`, () => liveExpenses(replica, ledgerId))
  const back = { name: 'ledger' as const, ledgerId }
  if (!ledgerLoaded || !expensesLoaded) return <Screen back={back} title={t('expenseForm.editTitle')} children={null} />
  const me = user && myMember(summary?.members ?? [], user.id)
  const latest = expenses?.find((expense) => expense.id === expenseId)
  if (!summary || !user || !me || !latest) {
    return (
      <Screen back={summary ? back : { name: 'home' }} title={t('expenseForm.editTitle')}>
        <p className="empty">{t('expenseForm.failedGone')}</p>
      </Screen>
    )
  }
  return <EditExpenseFrom key={expenseId} latest={latest} summary={summary} user={user} me={me} />
}

// What the Usuario is editing stays put when a newer version reaches the replica:
// the form tells them, and only loads it when they say so.
function EditExpenseFrom({ latest, summary, user, me }: { latest: ExpenseView; summary: LedgerSummary; user: User; me: Member }) {
  const { t } = useTranslation()
  const [editing, setEditing] = useState(latest)
  if (editing.version === undefined || editing.deletedAt) {
    return (
      <Screen back={{ name: 'ledger', ledgerId: latest.ledgerId }} title={t('expenseForm.editTitle')}>
        <p className="empty">{editing.deletedAt ? t('expenseForm.deleted') : t('expenseForm.notSynced')}</p>
      </Screen>
    )
  }
  return (
    <ExpenseForm
      key={editing.version}
      summary={summary}
      user={user}
      me={me}
      purpose={{ kind: 'edit', expense: editing, version: editing.version, latest, loadLatest: () => setEditing(latest) }}
    />
  )
}

type Purpose =
  | { kind: 'new' }
  | { kind: 'fix'; id: string; reason: RejectionReason; draft: NewExpenseRecord }
  | { kind: 'edit'; expense: ExpenseView; version: number; latest: ExpenseView; loadLatest: () => void }

function ExpenseForm({ summary, user, me, purpose }: { summary: LedgerSummary; user: User; me: Member; purpose: Purpose }) {
  const { t, i18n } = useTranslation()
  const { ledger, members } = summary
  const online = useOnline()
  const ids = useId()
  const [used] = useLive(`usedCurrencies:${user.id}`, () => liveUsedCurrencies(replica, user.id))
  const draft = purpose.kind === 'fix' ? purpose.draft : purpose.kind === 'edit' ? purpose.expense : undefined
  const editing = purpose.kind === 'edit' ? purpose : undefined

  // Only an Expense the server has can include former members: a fixed record is
  // new, so it simply cannot keep them.
  const former = editing ? formerShares(editing.expense, members) : noFormerShares
  const active = activeMembers(members, me)
  const formerIn = (shares: ReadonlyMap<string, unknown>) => members.filter((m) => !isActive(m) && shares.has(m.id))
  const payers = [...active, ...formerIn(former.paid)]
  const splitting = [...active, ...formerIn(former.owed)]
  const formerNames = [...new Set([...formerIn(former.paid), ...formerIn(former.owed)].map((m) => m.name))]
  const nameOf = (id: string) => (id === me.id ? t('common.you') : (members.find((m) => m.id === id)?.name ?? ''))

  const groupCurrency = ledger.kind === 'group' ? ledger.currency : undefined
  const [chosenCurrency, setCurrency] = useState(draft?.currency ?? user.preferredCurrency)
  const currency = groupCurrency ?? chosenCurrency
  const [amountText, setAmountText] = useState(draft ? formatAmountInput(draft.total, draft.currency) : '')
  const [description, setDescription] = useState(draft?.description ?? '')
  const [category, setCategory] = useState<Category | undefined>(draft?.category)
  const [date, setDate] = useState(draft?.date ?? todayISO())
  const [manyPayersChosen, setManyPayers] = useState((draft?.paidShares.length ?? 1) > 1)
  // A former member's Aporte cannot go away, so nobody else can pay it all.
  const manyPayers = manyPayersChosen || former.paid.size > 0
  const [chosenPayer, setPayer] = useState(draft?.paidShares[0]?.memberId ?? me.id)
  const payer = active.some((m) => m.id === chosenPayer) ? chosenPayer : me.id
  const [paidText, setPaidText] = useState(() => initialPaidText(draft))
  const [method, setMethod] = useState<SplitMethod>(draft?.splitMethod ?? 'equal')
  const [included, setIncluded] = useState<Record<string, boolean>>(() =>
    Object.fromEntries(active.map((m) => [m.id, draft ? draft.owedShares.some((o) => o.memberId === m.id) : true])),
  )
  const [values, setValues] = useState<SplitValues>(() =>
    initialSplitValues(
      active.map((m) => m.id),
      draft,
    ),
  )
  const [touched, setTouched] = useState(false)
  const [saving, setSaving] = useState(false)
  const [failure, setFailure] = useState<UpdateOutcome>()

  const read = readAmount(amountText, currency)
  const total = read.ok ? read.value : 0
  // A form always shows every decimal of the currency.
  const money = (minor: number) => formatMoney(minor, currency, { trim: false })

  // A person's field that cannot be read blocks saving and says why, as the
  // total does; it never counts as zero. Fields that do not count are not read:
  // a former member's, or the adjustment of someone left out.
  const valueProblems = new Map<string, string>()
  const participants: SplitParticipant[] = splitting.map((m) => {
    const kept = former.owed.get(m.id)
    if (kept) return formerParticipant(kept, method)
    if (method === 'equal') return { memberId: m.id, included: included[m.id] ?? false, value: 0 }
    const on = method !== 'adjustment' || (included[m.id] ?? false)
    const read = readValue(values[method][m.id] ?? '', method, currency)
    if (!read.ok && on) valueProblems.set(m.id, valueError(read.error, method, currency, t))
    const value = read.ok ? read.value : 0
    switch (method) {
      case 'adjustment':
        return { memberId: m.id, included: on, value }
      case 'weights':
        return { memberId: m.id, included: value > 0, value }
      default:
        return { memberId: m.id, included: value !== 0, value }
    }
  })

  const preview = previewOwedShares(total, method, participants)
  const shareOf = (id: string) => preview.shares.find((x) => x.memberId === id)
  const formerChange = preview.owedShares && changedFormerShare(former, preview.owedShares)

  const paidProblems = new Map<string, string>()
  const paidShares = manyPayers
    ? payers
        .map((m) => {
          const kept = former.paid.get(m.id)
          if (kept !== undefined) return { memberId: m.id, amount: kept }
          const read = readValue(paidText[m.id] ?? '', 'paid', currency)
          if (!read.ok) paidProblems.set(m.id, valueError(read.error, 'paid', currency, t))
          return { memberId: m.id, amount: read.ok ? read.value : 0 }
        })
        .filter((p) => p.amount > 0)
    : [{ memberId: payer, amount: total }]
  const paidLeft = total - paidShares.reduce((sum, p) => sum + p.amount, 0)

  // The first field that cannot be read, named, for the lines that are not next
  // to it: the status of the split or of the payers, and the dock.
  const problemOf = (problems: ReadonlyMap<string, string>, field: 'value' | 'paid') => {
    const [id, problem] = [...problems][0] ?? []
    if (!id || !problem) return undefined
    if (id === me.id) return field === 'value' ? t('expenseForm.valueProblemYours', { problem }) : t('expenseForm.paidProblemYours', { problem })
    const name = nameOf(id)
    return field === 'value' ? t('expenseForm.valueProblem', { name, problem }) : t('expenseForm.paidProblem', { name, problem })
  }
  const valueProblem = problemOf(valueProblems, 'value')
  const paidProblem = problemOf(paidProblems, 'paid')

  const percentSum = participants.reduce((sum, p) => sum + (method === 'percentage' ? p.value : 0), 0)
  const errors = {
    amount: amountError(read, currency, t),
    description: description.trim() === '' ? t('expenseForm.descriptionRequired') : undefined,
  }
  const splitOk = !valueProblem && !preview.issue && preview.unassigned === 0 && !formerChange
  const valid = !errors.amount && !errors.description && splitOk && !paidProblem && paidLeft === 0

  // Someone else got there first, whether the server said so or a sync brought it.
  const changed =
    !!editing && !saving && (failure === 'conflict' || editing.latest.version !== editing.version || !!editing.latest.deletedAt)
  const sawNewVersion = !!editing && editing.latest.version !== editing.version
  const canSave = valid && !saving && !changed && (online || !editing)

  useEffect(() => {
    if (changed) window.scrollTo(0, 0)
  }, [changed])

  const setValue = (id: string, text: string) => setValues((v) => ({ ...v, [method]: { ...v[method], [id]: text } }))

  const save = async () => {
    setTouched(true)
    if (!canSave) return
    setSaving(true)
    setFailure(undefined)
    const content = {
      description: description.trim(),
      date,
      currency,
      total,
      category,
      splitMethod: method,
      paidShares,
      owedShares: toOwedShares(preview.owedShares ?? []),
    }
    if (editing) {
      const outcome = await updateExpense(api, editing.expense.id, { ...content, expectedVersion: editing.version })
      // Whatever came back, the replica is behind the server now. Waiting for it
      // also keeps the User's own edit from looking like someone else's.
      await engine.sync()
      if (outcome === 'saved') {
        toast(t('expenseForm.edited'))
        navigate({ name: 'ledger', ledgerId: ledger.id }, { replace: true })
        return
      }
      setFailure(outcome)
      setSaving(false)
      return
    }
    await engine.addExpense({ ...content, ledgerId: ledger.id }, purpose.kind === 'fix' ? { replacing: purpose.id } : undefined)
    toast(online ? t('expenseForm.saved') : t('expenseForm.savedOffline'))
    navigate({ name: 'ledger', ledgerId: ledger.id }, { replace: true })
  }

  // How much of the total the Cuotas already cover, as ten blocks.
  const progress = method === 'percentage' ? percentSum / 10000 : total > 0 ? preview.assigned / total : 0
  const remainderShares = preview.shares.filter((x) => x.remainder > 0)
  const remainderTotal = remainderShares.reduce((sum, x) => sum + x.remainder, 0)

  let closing: { tone: 'even' | 'problem' | 'quiet'; text: string }
  if (total <= 0) closing = { tone: 'quiet', text: errors.amount ?? t('expenseForm.amountRequired') }
  else if (valueProblem) closing = { tone: 'problem', text: valueProblem }
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
  else if (formerChange)
    closing = {
      tone: 'problem',
      text: t('expenseForm.formerShareChanges', { name: nameOf(formerChange.memberId), from: money(formerChange.from), to: money(formerChange.to) }),
    }
  else closing = { tone: 'even', text: t('expenseForm.closes') }

  // What stops the save, said next to the button so it never looks dead.
  const blocking =
    errors.amount ??
    errors.description ??
    (!splitOk ? closing.text : undefined) ??
    paidProblem ??
    (paidLeft > 0
      ? t('expenseForm.payersLeft', { amount: money(paidLeft) })
      : paidLeft < 0
        ? t('expenseForm.payersOver', { amount: money(-paidLeft) })
        : undefined)

  const backRoute = { name: 'ledger' as const, ledgerId: ledger.id }
  const title = editing ? t('expenseForm.editTitle') : purpose.kind === 'fix' ? t('expenseForm.fixTitle') : t('expenseForm.title')
  const saveLabel = saving ? t('expenseForm.saving') : editing ? t('expenseForm.saveEdit') : online ? t('expenseForm.save') : t('expenseForm.saveOffline')

  return (
    <Screen
      back={backRoute}
      title={title}
      dock={
        <div className="dock-stack">
          {changed && editing ? (
            <>
              {editing.latest.deletedAt ? (
                <a className="button button-primary button-block" href={href(backRoute)}>
                  {t('expenseForm.backToLedger')}
                </a>
              ) : (
                <>
                  <p className="dock-note">{t('expenseForm.conflictDiscards')}</p>
                  <button type="button" className="button button-primary button-block" disabled={!sawNewVersion} onClick={editing.loadLatest}>
                    {t('expenseForm.conflictLoad')}
                  </button>
                </>
              )}
            </>
          ) : (
            <>
              {!online && <p className="dock-note">{editing ? t('expenseForm.editOffline') : t('expenseForm.offlineNote')}</p>}
              {failure && (
                <p className="dock-note tone-problem" role="alert">
                  {failureText(failure, t)}
                </p>
              )}
              {blocking && (
                <p className={`dock-note${touched ? ' tone-problem' : ''}`} id={`${ids}-blocking`}>
                  {blocking}
                </p>
              )}
              <button
                type="button"
                className="button button-primary button-block"
                onClick={() => void save()}
                disabled={!!editing && !online}
                aria-disabled={!canSave}
                aria-describedby={blocking ? `${ids}-blocking` : undefined}
                title={editing && !online ? t('common.onlineOnly') : undefined}
              >
                <Icon name="check" />
                {saveLabel}
              </button>
            </>
          )}
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
        <p className="form-context">
          {ledger.kind === 'group' ? <GroupGlyph size={20} /> : <Figure state={figureState(active[1] ?? me)} size={20} />}
          {ledgerName(summary, user.id)}
        </p>

        {changed && editing && <ConflictNotice latest={editing.latest} newVersion={sawNewVersion} nameOf={nameOf} />}

        {purpose.kind === 'fix' && <p className="notice notice-problem">{rejectionText(purpose, groupCurrency, t)}</p>}

        {formerNames.length > 0 && (
          <p className="notice">
            {t('expenseForm.formerNote', { count: formerNames.length, names: new Intl.ListFormat(i18n.language).format(formerNames) })}
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
                <CurrencyOptions value={currency} preferred={user.preferredCurrency} used={used} />
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
                {active.map((m) => (
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
                {payers.map((m) => {
                  const problem = touched ? paidProblems.get(m.id) : undefined
                  return (
                    <li key={m.id} className="p-row p-row-input">
                      <Figure state={figureState(m)} size={24} />
                      <span className="p-name">{nameOf(m.id)}</span>
                      <input
                        className="input input-amount"
                        inputMode="decimal"
                        placeholder="0"
                        aria-label={t('expenseForm.valueFor', { name: nameOf(m.id) })}
                        value={paidText[m.id] ?? ''}
                        disabled={former.paid.has(m.id)}
                        onChange={(e) => setPaidText((p) => ({ ...p, [m.id]: e.target.value }))}
                        aria-invalid={!!problem}
                        aria-describedby={problem ? `${ids}-paid-${m.id}` : undefined}
                      />
                      {problem && (
                        <p className="field-error p-error" id={`${ids}-paid-${m.id}`}>
                          {problem}
                        </p>
                      )}
                    </li>
                  )
                })}
              </ul>
              <div className="split-status">
                <p className={`closing tone-${!paidProblem && paidLeft === 0 ? 'even' : 'problem'}`}>
                  {!paidProblem && paidLeft === 0 && <Icon name="check" size={18} />}
                  <span className="closing-text">
                    {paidProblem ??
                      (paidLeft === 0
                        ? t('expenseForm.payersOk')
                        : paidLeft > 0
                          ? t('expenseForm.payersLeft', { amount: money(paidLeft) })
                          : t('expenseForm.payersOver', { amount: money(-paidLeft) }))}
                  </span>
                </p>
                {former.paid.size === 0 && (
                  <button type="button" className="button button-small button-quiet" onClick={() => setManyPayers(false)}>
                    {t('expenseForm.onePayer')}
                  </button>
                )}
              </div>
            </>
          )}
        </fieldset>

        <fieldset className="field">
          <legend>{t('expenseForm.split')}</legend>
          <div className="chips" role="radiogroup" aria-label={t('expenseForm.splitMethod')}>
            {methodsFor(draft?.splitMethod, former).map((m) => (
              <label key={m} className="chip">
                <input type="radio" name={`${ids}-method`} checked={method === m} onChange={() => setMethod(m)} />
                <span>{t(`splitMethods.${m}`)}</span>
              </label>
            ))}
          </div>
          <p className="hint">
            {t(`splitHints.${method}`)} {former.owed.size > 0 && t('expenseForm.formerMethods')}
          </p>

          <ul className={`people participants participants-${method}`} aria-label={t('expenseForm.participants')}>
            {splitting.map((m) => {
              const share = shareOf(m.id)
              const kept = former.owed.has(m.id)
              const checkable = method === 'equal' || method === 'adjustment'
              const on = participants.find((p) => p.memberId === m.id)?.included ?? false
              // Once the split closes, a Cuota of zero is real (1 cent among three).
              const owes = !!share && on && (share.amount !== 0 || !!preview.owedShares)
              const problem = touched ? valueProblems.get(m.id) : undefined
              return (
                <li key={m.id} className={`p-row${on ? '' : ' is-out'}`}>
                  {checkable ? (
                    <label className="p-check">
                      <input
                        type="checkbox"
                        checked={kept || (included[m.id] ?? false)}
                        disabled={kept}
                        onChange={(e) => setIncluded((x) => ({ ...x, [m.id]: e.target.checked }))}
                        aria-label={t('expenseForm.includes', { name: nameOf(m.id) })}
                      />
                      <Figure state={figureState(m)} size={24} />
                    </label>
                  ) : (
                    <Figure state={figureState(m)} size={24} />
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
                        disabled={kept || (method === 'adjustment' && !included[m.id])}
                        onChange={(e) => setValue(m.id, e.target.value)}
                        aria-invalid={!!problem}
                        aria-describedby={problem ? `${ids}-value-${m.id}` : undefined}
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
                  {problem && (
                    <p className="field-error p-error" id={`${ids}-value-${m.id}`}>
                      {problem}
                    </p>
                  )}
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
            <span className="closing-text">{closing.text}</span>
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

// The version that won, so the User sees it before deciding. Nothing they typed
// is lost until they choose to load it.
function ConflictNotice({ latest, newVersion, nameOf }: { latest: ExpenseView; newVersion: boolean; nameOf: (id: string) => string }) {
  const { t } = useTranslation()
  const money = (minor: number) => formatMoney(minor, latest.currency, { trim: false })
  const shares = (list: readonly { memberId: string; amount: number }[]) => list.map((s) => `${nameOf(s.memberId)} ${money(s.amount)}`).join(', ')
  return (
    <div className="notice" role="alert">
      <p className="notice-problem">{t('expenseForm.conflict')}</p>
      {latest.deletedAt ? (
        <p>{t('expenseForm.conflictDeleted')}</p>
      ) : !newVersion ? (
        <p>{t('expenseForm.conflictWaiting')}</p>
      ) : (
        <>
          <p>{t('expenseForm.conflictNow')}</p>
          <p>{[latest.description, money(latest.total), formatDay(latest.date)].join(' · ')}</p>
          <p>{t('expenseForm.conflictPaid', { shares: shares(latest.paidShares) })}</p>
          <p>{t('expenseForm.conflictOwed', { shares: shares(latest.owedShares) })}</p>
        </>
      )}
    </div>
  )
}

function failureText(failure: UpdateOutcome, t: ReturnType<typeof useTranslation>['t']): string {
  switch (failure) {
    case 'currency':
      return t('expenseForm.failedCurrency')
    case 'participants':
      return t('expenseForm.failedParticipants')
    case 'gone':
      return t('expenseForm.failedGone')
    case 'offline':
      return t('expenseForm.failedOffline')
    default:
      return t('expenseForm.failed')
  }
}

function rejectionText(
  { reason, draft }: { reason: RejectionReason; draft: NewExpenseRecord },
  groupCurrency: string | undefined,
  t: ReturnType<typeof useTranslation>['t'],
): string {
  if (reason === 'currency_mismatch' && groupCurrency) return t('expenseForm.rejectedCurrency', { from: draft.currency, to: groupCurrency })
  if (reason === 'invalid_participants') return t('expenseForm.rejectedParticipants')
  return t('expenseForm.rejectedOther')
}
