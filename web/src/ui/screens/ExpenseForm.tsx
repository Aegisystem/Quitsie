import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { api, type Category, type Member, type NewExpense as NewExpenseRecord, type SplitMethod, type User } from '../../api/client'
import { liveExpenses, liveHome, liveLedger, liveRecords, type ExpenseView, type LedgerSummary, type RejectionReason } from '../../local'
import { CategoryGlyph, Figure, figureState, GroupGlyph, Icon } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { amountError, balanceError } from '../forms/amountError'
import { CurrencyOptions } from '../forms/CurrencyOptions'
import { engine, ledgerName, myMember, replica, useLive } from '../data'
import { suggestCategory } from '../../domain'
import { liveUsedCurrencies, useCategoryHistory, useUser } from '../forms/data'
import {
  balanceRecords,
  changedFormerShare,
  formerParticipant,
  formerShares,
  initialPaidText,
  initialSplitValues,
  methodsFor,
  noFormerShares,
  switchMethod,
  toOwedShares,
  type SplitValues,
} from '../forms/expense'
import { payerStatus, payerSummary, splitStatus, splitSummary, type Say } from '../forms/splitSummary'
import { activeMembers, isActive } from '../forms/people'
import { afterEdit, updateExpense, type UpdateOutcome } from '../forms/updateExpense'
import { problemPointer, readValue, valueError } from '../forms/values'
import { categories } from '../types'
import { previewOwedShares, type SplitParticipant } from '../split'
import { formatDay, todayISO } from '../dates'
import { AmountInput } from '../forms/AmountInput'
import { useChosenChipInSight } from '../forms/chosenChip'
import { amountPlaceholder, formatAmountInput, formatMoney, readAmount } from '../money'
import { closeOver, href, navigate, openOver, type ExpensePart, type Route } from '../router'
import { PaidScreen, SplitScreen, SummaryRow, type Over } from './ExpenseSplit'

export function NewExpense({ ledgerId, fromRejected, part }: { ledgerId?: string; fromRejected?: string; part?: ExpensePart }) {
  if (!ledgerId) return <PickLedger />
  // Re-mount when the ledger changes so the form starts clean.
  return <NewExpenseIn key={ledgerId} ledgerId={ledgerId} fromRejected={fromRejected} part={part} />
}

function NewExpenseIn({ ledgerId, fromRejected, part }: { ledgerId: string; fromRejected?: string; part?: ExpensePart }) {
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
  return <ExpenseForm summary={summary} user={user} me={me} purpose={purpose} part={part} />
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
export function EditExpense({ ledgerId, expenseId, part }: { ledgerId: string; expenseId: string; part?: ExpensePart }) {
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
  return <EditExpenseFrom key={expenseId} latest={latest} summary={summary} user={user} me={me} part={part} />
}

// What the Usuario is editing stays put when a newer version reaches the replica:
// the form tells them, and only loads it when they say so.
function EditExpenseFrom({
  latest,
  summary,
  user,
  me,
  part,
}: {
  latest: ExpenseView
  summary: LedgerSummary
  user: User
  me: Member
  part?: ExpensePart
}) {
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
      part={part}
    />
  )
}

type Purpose =
  | { kind: 'new' }
  | { kind: 'fix'; id: string; reason: RejectionReason; draft: NewExpenseRecord }
  | { kind: 'edit'; expense: ExpenseView; version: number; latest: ExpenseView; loadLatest: () => void }

function ExpenseForm({
  summary,
  user,
  me,
  purpose,
  part,
}: {
  summary: LedgerSummary
  user: User
  me: Member
  purpose: Purpose
  part?: ExpensePart
}) {
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
  // A Category picked by hand, or the one the Expense already had, is never
  // replaced by a suggestion; until then the description suggests one.
  const [categoryChosen, setCategoryChosen] = useState(draft?.category !== undefined)
  const categoryHistory = useCategoryHistory(user.id)
  const suggestedCategory = categoryChosen ? undefined : suggestCategory(description, categoryHistory)
  const shownCategory = categoryChosen ? category : suggestedCategory
  const categoriesRow = useChosenChipInSight(shownCategory)
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
  const [values, setValues] = useState<SplitValues>(() => initialSplitValues(draft))
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

  // The lines away from the fields (the status of the split or of the payers,
  // and the dock) point at the first one that cannot be read, by name.
  const problemOf = (problems: ReadonlyMap<string, string>, field: 'value' | 'paid') => {
    const [id] = [...problems.keys()]
    if (id === undefined) return undefined
    return problemPointer(field, id === me.id ? undefined : nameOf(id), t)
  }
  const valueProblem = problemOf(valueProblems, 'value')
  const paidProblem = problemOf(paidProblems, 'paid')

  const percentSum = participants.reduce((sum, p) => sum + (method === 'percentage' ? p.value : 0), 0)
  const splitOk = !valueProblem && !preview.issue && preview.unassigned === 0 && !formerChange
  const owedShares = toOwedShares(preview.owedShares ?? [])
  // Only once the split and the payers close is it known whose Balance moves, and how far.
  const closes = splitOk && !paidProblem && paidLeft === 0
  const errors = {
    amount:
      amountError(read, currency, t) ??
      (closes
        ? balanceError(
            summary.balances,
            { added: balanceRecords({ currency, paidShares, owedShares }), removed: editing && balanceRecords(editing.expense) },
            { me: me.id, nameOf },
            t,
          )
        : undefined),
    description: description.trim() === '' ? t('expenseForm.descriptionRequired') : undefined,
  }
  const valid = !errors.amount && !errors.description && closes

  // Someone else got there first, whether the server said so or a sync brought it.
  const changed =
    !!editing && !saving && (failure === 'conflict' || editing.latest.version !== editing.version || !!editing.latest.deletedAt)
  const sawNewVersion = !!editing && editing.latest.version !== editing.version
  const canSave = valid && !saving && !changed && (online || !editing)

  useEffect(() => {
    if (changed) window.scrollTo(0, 0)
  }, [changed])

  // The form's own address, under the parts opened over it.
  const formRoute: Route = editing
    ? { name: 'editExpense', ledgerId: ledger.id, expenseId: editing.expense.id }
    : { name: 'newExpense', ledgerId: ledger.id, fromRejected: purpose.kind === 'fix' ? purpose.id : undefined }
  const paidRow = useRef<HTMLAnchorElement>(null)
  const splitRow = useRef<HTMLAnchorElement>(null)
  // A part starts at the top. Closing it puts the form back where it was, with
  // the focus on the line that opened it.
  const formScroll = useRef(0)
  const shownPart = useRef(part)
  useLayoutEffect(() => {
    const before = shownPart.current
    shownPart.current = part
    if (before === part) return
    if (part) {
      window.scrollTo(0, 0)
      return
    }
    window.scrollTo(0, formScroll.current)
    if (before) (before === 'paid' ? paidRow : splitRow).current?.focus({ preventScroll: true })
  }, [part])
  const open = (which: ExpensePart) => {
    formScroll.current = window.scrollY
    openOver({ ...formRoute, part: which })
  }

  const setValue = (id: string, text: string) => setValues((v) => ({ ...v, [method]: { ...v[method], [id]: text } }))
  // Any change the User makes: every field of the form reports it with a change
  // event, and the few controls that do not call it themselves.
  const edited = () => setFailure(afterEdit)

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
      category: shownCategory,
      splitMethod: method,
      paidShares,
      owedShares,
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

  const say: Say = { t, nameOf, money }
  const closing = splitStatus(
    { total, method, issue: preview.issue, unassigned: preview.unassigned, percentSum, valueProblem, formerChange },
    say,
  )
  const payersClosing = payerStatus({ left: paidLeft, paidProblem }, say)
  const payersProblem = payersClosing.tone === 'problem' ? payersClosing.text : undefined
  const takingPart = participants.filter((p) => p.included).map((p) => p.memberId)
  const payerLine = payerSummary({ many: manyPayers, paid: paidShares, problem: payersProblem, me: me.id }, say)
  const splitLine = splitSummary(
    {
      method,
      takingPart,
      everyone: splitting.length,
      shares: preview.owedShares?.map(({ participantId, amount }) => ({ memberId: participantId, amount })),
      problem: closing.tone === 'problem' ? closing.text : undefined,
      me: me.id,
    },
    say,
  )

  // What stops the save, said next to the button so it never looks dead.
  const blocking = errors.amount ?? errors.description ?? (!splitOk ? closing.text : undefined) ?? payersProblem

  // A new method keeps who takes part and starts from the split on screen.
  const chooseMethod = (to: SplitMethod) => {
    const shares = splitOk && preview.owedShares ? new Map(preview.owedShares.map((s) => [s.participantId, s.amount])) : undefined
    const next = switchMethod(to, { values, included, activeIds: active.map((m) => m.id), takingPart, shares, currency })
    setValues(next.values)
    setIncluded(next.included)
    setMethod(to)
  }

  const formerNotice = formerNames.length > 0 && (
    <p className="notice">{t('expenseForm.formerNote', { count: formerNames.length, names: new Intl.ListFormat(i18n.language).format(formerNames) })}</p>
  )
  const over: Over = {
    back: formRoute,
    close: () => closeOver(formRoute),
    onEdit: edited,
    context: [description.trim() || ledgerName(summary, user.id), total > 0 ? money(total) : undefined].filter(Boolean).join(' · '),
    notices: formerNotice,
  }

  if (part === 'paid') {
    return (
      <PaidScreen
        over={over}
        active={active}
        payers={payers}
        many={manyPayers}
        payer={payer}
        formerPaid={former.paid}
        paidText={paidText}
        paidProblems={paidProblems}
        status={payersClosing}
        touched={touched}
        currency={currency}
        me={me.id}
        nameOf={nameOf}
        onPick={(id) => {
          setPayer(id)
          edited()
          over.close()
        }}
        onMany={setManyPayers}
        onPaidText={(id, text) => setPaidText((p) => ({ ...p, [id]: text }))}
      />
    )
  }
  if (part === 'split') {
    return (
      <SplitScreen
        over={over}
        methods={methodsFor(draft?.splitMethod, former)}
        method={method}
        onMethod={chooseMethod}
        formerMethods={former.owed.size > 0}
        members={splitting}
        participants={participants}
        preview={preview}
        kept={new Set(former.owed.keys())}
        values={values}
        onValue={setValue}
        included={included}
        onInclude={(ids, on) => setIncluded((x) => ({ ...x, ...Object.fromEntries(ids.map((id) => [id, on])) }))}
        valueProblems={valueProblems}
        status={closing}
        progress={progress}
        remainder={
          remainderTotal > 0 && !preview.issue ? t('expenseForm.remainder', { count: remainderShares.length, amount: money(remainderTotal) }) : undefined
        }
        touched={touched}
        currency={currency}
        nameOf={nameOf}
        money={money}
      />
    )
  }

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
        onChange={edited}
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

        {formerNotice}

        <div className="field field-amount">
          <label htmlFor={`${ids}-amount`}>{t('expenseForm.amount')}</label>
          <div className="amount-input">
            <AmountInput
              id={`${ids}-amount`}
              currency={currency}
              autoComplete="off"
              placeholder={amountPlaceholder(currency)}
              value={amountText}
              onChange={setAmountText}
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
          <legend>
            {t('categories.label')}
            {suggestedCategory && <span className="legend-note">{t('categories.suggested')}</span>}
          </legend>
          <div ref={categoriesRow} className="chips chips-scroll" role="radiogroup" aria-label={t('categories.label')}>
            {categories.map((c) => (
              <label key={c} className="chip chip-glyph">
                <input
                  type="radio"
                  name={`${ids}-category`}
                  checked={shownCategory === c}
                  onChange={() => {
                    setCategory(c)
                    setCategoryChosen(true)
                  }}
                  onClick={() => {
                    if (shownCategory !== c) return
                    setCategoryChosen(true)
                    // Tapping the suggestion takes it; tapping a chosen one clears it.
                    if (!categoryChosen) {
                      setCategory(c)
                      return
                    }
                    setCategory(undefined)
                    edited()
                  }}
                />
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
          <legend>{t('expenseForm.split')}</legend>
          <ul className="rows split-lines">
            <li>
              <SummaryRow
                rowRef={paidRow}
                label={t('expenseForm.paidTitle')}
                line={payerLine}
                glyph={manyPayers ? <GroupGlyph size={26} /> : <Figure state={figureState(members.find((m) => m.id === payer))} size={26} />}
                href={href({ ...formRoute, part: 'paid' })}
                onOpen={() => open('paid')}
              />
            </li>
            <li>
              <SummaryRow
                rowRef={splitRow}
                label={t('expenseForm.splitTitle')}
                line={splitLine}
                glyph={
                  takingPart.length === 1 ? <Figure state={figureState(members.find((m) => m.id === takingPart[0]))} size={26} /> : <GroupGlyph size={26} />
                }
                href={href({ ...formRoute, part: 'split' })}
                onOpen={() => open('split')}
              />
            </li>
          </ul>
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
    case 'balance':
      return t('expenseForm.failedBalance')
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
