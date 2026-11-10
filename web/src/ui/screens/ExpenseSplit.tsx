import { type CSSProperties, type ReactNode, type Ref, useId } from 'react'
import { useTranslation } from 'react-i18next'
import type { Member, SplitMethod } from '../../api/client'
import { Figure, figureState, GroupGlyph, Icon } from '../components/icons'
import { Screen } from '../components/Shell'
import { AmountInput } from '../forms/AmountInput'
import { useChosenChipInSight } from '../forms/chosenChip'
import type { SplitValues } from '../forms/expense'
import type { Status, SummaryLine } from '../forms/splitSummary'
import { readValue } from '../forms/values'
import type { Route } from '../router'
import type { SplitParticipant, SplitPreview } from '../split'

// The split of an Expense, out of the way of the form (DESIGN.md, Reparto): the
// form shows one line for who paid and one for how it is divided, and each opens
// a screen of its own over the form. The state stays in ExpenseForm, so going
// back and forth loses nothing.

// What both screens share: the way back to the form and what is being split.
export interface Over {
  // The form, for the address of the back button.
  back: Route
  // Back to the form, through the history (closeOver).
  close: () => void
  // Any change the Usuario makes, as the form counts them (afterEdit).
  onEdit: () => void
  // "Cena · $120.00", so the Usuario knows what they are splitting.
  context: string
  // Notices that also concern this part, such as former members.
  notices?: ReactNode
}

// A line of the form that opens a part of the split.
export function SummaryRow({
  label,
  line,
  glyph,
  href,
  onOpen,
  rowRef,
}: {
  label: string
  line: SummaryLine
  glyph: ReactNode
  href: string
  onOpen: () => void
  rowRef: Ref<HTMLAnchorElement>
}) {
  return (
    <a
      ref={rowRef}
      className="row row-link summary-row"
      href={href}
      onClick={(event) => {
        event.preventDefault()
        onOpen()
      }}
    >
      <span className="row-glyph">{glyph}</span>
      <span className="row-main">
        <span className="sr-only">{label}: </span>
        <span className="row-title">{line.title}</span>
        {line.detail && <span className={`row-sub${line.problem ? ' tone-problem' : ''}`}>{line.detail}</span>}
      </span>
      <span className="row-end">
        <Icon name="chevron" size={20} />
      </span>
    </a>
  )
}

function Closing({ status }: { status: Status }) {
  return (
    <p className={`closing tone-${status.tone}`} role="status">
      {status.tone === 'even' && <Icon name="check" size={18} />}
      <span className="closing-text">{status.text}</span>
    </p>
  )
}

function Done({ over, children }: { over: Over; children?: ReactNode }) {
  const { t } = useTranslation()
  return (
    <div className="dock-stack">
      {children}
      <button type="button" className="button button-primary button-block" onClick={over.close}>
        {t('expenseForm.done')}
      </button>
    </div>
  )
}

function Part({ over, title, dock, children }: { over: Over; title: string; dock?: ReactNode; children: ReactNode }) {
  return (
    <Screen back={over.back} onBack={over.close} title={title} dock={dock}>
      <form className="form" noValidate onChange={over.onEdit} onSubmit={(event) => event.preventDefault()}>
        <p className="form-context">{over.context}</p>
        {over.notices}
        {children}
      </form>
    </Screen>
  )
}

export function PaidScreen({
  over,
  active,
  payers,
  many,
  payer,
  formerPaid,
  paidText,
  paidProblems,
  status,
  touched,
  currency,
  me,
  nameOf,
  onPick,
  onMany,
  onPaidText,
}: {
  over: Over
  // Who can pay alone: the active Members, the User first.
  active: readonly Member[]
  // The rows when several paid: the active Members and any former payer.
  payers: readonly Member[]
  many: boolean
  payer: string
  formerPaid: ReadonlyMap<string, number>
  paidText: Record<string, string>
  paidProblems: ReadonlyMap<string, string>
  status: Status
  touched: boolean
  currency: string
  me: string
  nameOf: (memberId: string) => string
  // One person paid it all: choosing them closes the screen.
  onPick: (memberId: string) => void
  onMany: (many: boolean) => void
  onPaidText: (memberId: string, text: string) => void
}) {
  const { t } = useTranslation()
  const ids = useId()
  if (!many) {
    return (
      <Part over={over} title={t('expenseForm.paidTitle')}>
        <ul className="rows choice-rows">
          {active.map((m) => (
            <li key={m.id}>
              <button type="button" className="row row-link row-button" aria-pressed={payer === m.id} onClick={() => onPick(m.id)}>
                <span className="row-glyph">
                  <Figure state={figureState(m)} size={26} />
                </span>
                <span className="row-main">
                  <span className="row-title">{nameOf(m.id)}</span>
                </span>
                <span className="row-end">{payer === m.id && <Icon name="check" />}</span>
              </button>
            </li>
          ))}
          <li>
            <button
              type="button"
              className="row row-link row-button"
              onClick={() => {
                onMany(true)
                over.onEdit()
              }}
            >
              <span className="row-glyph">
                <GroupGlyph size={26} />
              </span>
              <span className="row-main">
                <span className="row-title">{t('expenseForm.severalPayers')}</span>
                <span className="row-sub">{t('expenseForm.severalPayersHint')}</span>
              </span>
              <span className="row-end">
                <Icon name="chevron" size={20} />
              </span>
            </button>
          </li>
        </ul>
      </Part>
    )
  }
  return (
    <Part
      over={over}
      title={t('expenseForm.paidTitle')}
      dock={
        <Done over={over}>
          {/* What is left to cover is not a mistake while the Usuario is still typing. */}
          <Closing status={status.tone === 'problem' && !touched ? { ...status, tone: 'quiet' } : status} />
        </Done>
      }
    >
      <p className="hint">{t('expenseForm.severalPayersHint')}</p>
      <ul className="people people-paid">
        {payers.map((m) => {
          const problem = touched ? paidProblems.get(m.id) : undefined
          return (
            <li key={m.id} className="p-row">
              <Figure state={figureState(m)} size={24} />
              <span className="p-name">{nameOf(m.id)}</span>
              <AmountInput
                className="input input-amount"
                currency={currency}
                placeholder="0"
                aria-label={m.id === me ? t('expenseForm.paidForYou') : t('expenseForm.paidFor', { name: nameOf(m.id) })}
                value={paidText[m.id] ?? ''}
                disabled={formerPaid.has(m.id)}
                onChange={(text) => onPaidText(m.id, text)}
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
      {formerPaid.size === 0 && (
        <button
          type="button"
          className="button button-small button-quiet button-start"
          onClick={() => {
            onMany(false)
            over.onEdit()
          }}
        >
          {t('expenseForm.onePayer')}
        </button>
      )}
    </Part>
  )
}

export function SplitScreen({
  over,
  methods,
  method,
  onMethod,
  formerMethods,
  members,
  participants,
  preview,
  kept,
  values,
  onValue,
  included,
  onInclude,
  valueProblems,
  status,
  progress,
  remainder,
  touched,
  currency,
  nameOf,
  money,
}: {
  over: Over
  methods: readonly SplitMethod[]
  method: SplitMethod
  onMethod: (method: SplitMethod) => void
  // A former member limits the methods, and the screen says so.
  formerMethods: boolean
  // Everyone in the list: the active Members and any former Participant.
  members: readonly Member[]
  participants: readonly SplitParticipant[]
  preview: SplitPreview
  // The former members, whose part cannot change.
  kept: ReadonlySet<string>
  values: SplitValues
  onValue: (memberId: string, text: string) => void
  included: Record<string, boolean>
  onInclude: (memberIds: readonly string[], on: boolean) => void
  valueProblems: ReadonlyMap<string, string>
  status: Status
  // How much of the total the Cuotas cover, 1 when they cover it all.
  progress: number
  // The note on the leftover minor units, when there are some.
  remainder?: string
  touched: boolean
  currency: string
  nameOf: (memberId: string) => string
  money: (minor: number) => string
}) {
  const { t } = useTranslation()
  const ids = useId()
  const methodsRow = useChosenChipInSight(method)
  const checkable = method === 'equal' || method === 'adjustment'
  const isIn = (memberId: string) => participants.find((p) => p.memberId === memberId)?.included ?? false
  const choosable = members.filter((m) => !kept.has(m.id))
  const allIn = choosable.every((m) => included[m.id])
  const inCount = members.filter((m) => isIn(m.id)).length

  // Weights go up and down one part at a time; zero leaves someone out.
  const step = (memberId: string, by: number) => {
    const read = readValue(values.weights[memberId] ?? '', 'weights', currency)
    const next = Math.max(0, (read.ok ? read.value : 0) + by)
    onValue(memberId, next === 0 ? '' : String(next))
    over.onEdit()
  }

  return (
    <Part
      over={over}
      title={t('expenseForm.splitTitle')}
      dock={
        <Done over={over}>
          <div className="meter" aria-hidden="true" style={{ '--p': Math.max(0, Math.min(1, progress)) } as CSSProperties}>
            {Array.from({ length: 10 }, (_, i) => (
              <i key={i} className={progress > 1 ? 'is-over' : ''} style={{ '--b': Math.max(0, Math.min(1, progress * 10 - i)) } as CSSProperties} />
            ))}
          </div>
          <Closing status={status} />
        </Done>
      }
    >
      <div ref={methodsRow} className="chips chips-scroll" role="radiogroup" aria-label={t('expenseForm.splitMethod')}>
        {methods.map((m) => (
          <label key={m} className="chip">
            <input type="radio" name={`${ids}-method`} checked={method === m} onChange={() => onMethod(m)} />
            <span>{t(`splitMethods.${m}`)}</span>
          </label>
        ))}
      </div>
      <p className="hint split-hint">
        {t(`splitHints.${method}`)} {formerMethods && t('expenseForm.formerMethods')}
      </p>

      <div className="split-list">
        {checkable && (
          <div className="list-head">
            <span>{t('expenseForm.participating', { count: inCount, everyone: members.length })}</span>
            <button
              type="button"
              className="button button-small button-quiet"
              onClick={() => {
                onInclude(
                  choosable.map((m) => m.id),
                  !allIn,
                )
                over.onEdit()
              }}
            >
              {allIn ? t('expenseForm.untickAll') : t('expenseForm.tickAll')}
            </button>
          </div>
        )}
        <ul className={`people participants participants-${method}`} aria-label={t('expenseForm.participants')}>
          {members.map((m) => {
            const name = nameOf(m.id)
            const share = preview.shares.find((x) => x.memberId === m.id)
            const fixed = kept.has(m.id)
            const on = isIn(m.id)
            // Once the split closes, a Cuota of zero is real (1 cent among three).
            const owes = !!share && on && (share.amount !== 0 || !!preview.owedShares)
            const problem = touched ? valueProblems.get(m.id) : undefined
            const field = {
              'aria-label': t('expenseForm.valueFor', { name }),
              'aria-invalid': !!problem,
              'aria-describedby': problem ? `${ids}-value-${m.id}` : undefined,
            }
            const text = method === 'equal' ? '' : (values[method][m.id] ?? '')
            return (
              <li key={m.id} className={`p-row${checkable && !on ? ' is-out' : ''}`}>
                {checkable ? (
                  <label className="p-check">
                    <input
                      type="checkbox"
                      checked={fixed || (included[m.id] ?? false)}
                      disabled={fixed}
                      onChange={(e) => onInclude([m.id], e.target.checked)}
                      aria-label={t('expenseForm.includes', { name })}
                    />
                    <Figure state={figureState(m)} size={24} />
                  </label>
                ) : (
                  <Figure state={figureState(m)} size={24} />
                )}
                <span className="p-name">{name}</span>
                {(method === 'exact' || method === 'adjustment') && (
                  <AmountInput
                    {...field}
                    className="input input-amount"
                    currency={currency}
                    allowNegative={method === 'adjustment'}
                    placeholder={method === 'adjustment' ? '+0' : '0'}
                    value={text}
                    disabled={fixed || (method === 'adjustment' && !included[m.id])}
                    onChange={(value) => onValue(m.id, value)}
                  />
                )}
                {method === 'percentage' && (
                  <span className="p-value">
                    <input
                      {...field}
                      className="input input-amount input-percent"
                      inputMode="decimal"
                      placeholder="0"
                      value={text}
                      disabled={fixed}
                      onChange={(e) => onValue(m.id, e.target.value)}
                    />
                    <span className="p-unit" aria-hidden="true">
                      %
                    </span>
                  </span>
                )}
                {method === 'weights' && (
                  <span className="stepper">
                    <button type="button" aria-label={t('expenseForm.lessPart', { name })} disabled={fixed || !on} onClick={() => step(m.id, -1)}>
                      <Icon name="minus" size={20} />
                    </button>
                    <input {...field} inputMode="numeric" placeholder="0" value={text} disabled={fixed} onChange={(e) => onValue(m.id, e.target.value)} />
                    <button type="button" aria-label={t('expenseForm.morePart', { name })} disabled={fixed} onClick={() => step(m.id, 1)}>
                      <Icon name="plus" size={20} />
                    </button>
                  </span>
                )}
                {method !== 'exact' && (
                  <span className="p-share" aria-label={t('expenseForm.shareFor', { name })}>
                    {owes ? money(share.amount) : '—'}
                    {share && share.remainder > 0 && <sup className="p-remainder">+{money(share.remainder)}</sup>}
                  </span>
                )}
                {problem && (
                  <p className="field-error p-error" id={`${ids}-value-${m.id}`}>
                    {problem}
                  </p>
                )}
              </li>
            )
          })}
        </ul>
      </div>
      {remainder && <p className="hint">{remainder}</p>}
    </Part>
  )
}
