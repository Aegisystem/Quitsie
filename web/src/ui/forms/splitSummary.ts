import type { TFunction } from 'i18next'
import type { SplitMethod } from '../../api/client'
import type { SplitIssue } from '../split'
import { formatPercent } from '../money'

// What the Expense form says about its split: a line for who paid and a line for
// how it is divided, each opening its own screen, and the status those screens
// show while the Usuario types.

export interface Say {
  t: TFunction
  // The User is "Tú"; everyone else, by name.
  nameOf: (memberId: string) => string
  money: (minor: number) => string
}

export interface Amount {
  memberId: string
  amount: number
}

// One of the two lines of the form. A problem takes the place of the amounts,
// so the line says what to fix.
export interface SummaryLine {
  title: string
  detail?: string
  problem: boolean
}

export interface Status {
  tone: 'even' | 'problem' | 'quiet'
  text: string
}

export function payerSummary(
  { many, paid, problem, me }: { many: boolean; paid: readonly Amount[]; problem?: string; me: string },
  { t, nameOf, money }: Say,
): SummaryLine {
  if (many && problem) return { title: t('expenseForm.summary.severalPaid'), detail: problem, problem: true }
  const [first] = paid
  if (!first) return { title: t('expenseForm.summary.severalPaid'), problem: false }
  if (paid.length === 1) {
    const { memberId, amount } = first
    const title = memberId === me ? t('expenseForm.summary.youPaid') : t('expenseForm.summary.paid', { name: nameOf(memberId) })
    return { title, detail: amount > 0 ? money(amount) : undefined, problem: false }
  }
  return {
    title: t('expenseForm.summary.paidBy', { count: paid.length }),
    detail: list(paid, nameOf, money),
    problem: false,
  }
}

export function splitSummary(
  {
    method,
    takingPart,
    everyone,
    shares,
    problem,
    me,
  }: {
    method: SplitMethod
    // Who is in the split now, in the order of the list.
    takingPart: readonly string[]
    // How many people the list has.
    everyone: number
    // The Cuotas, once the split closes.
    shares?: readonly Amount[]
    problem?: string
    me: string
  },
  { t, nameOf, money }: Say,
): SummaryLine {
  const count = takingPart.length
  const [only] = takingPart
  const label = t(`splitMethods.${method}`)
  const title =
    only === undefined
      ? t('expenseForm.summary.nobody')
      : count === 1
        ? only === me
          ? t('expenseForm.summary.allYours')
          : t('expenseForm.summary.allFor', { name: nameOf(only) })
        : count === everyone
          ? t('expenseForm.summary.amongAll', { method: label, count })
          : t('expenseForm.summary.amongSome', { method: label, count, everyone })
  if (problem) return { title, detail: problem, problem: true }
  const owed = shares?.filter((share) => takingPart.includes(share.memberId)) ?? []
  const [first] = owed
  if (!first) return { title, problem: false }
  if (owed.length === 1) return { title, detail: money(first.amount), problem: false }
  const same = owed.every((share) => share.amount === first.amount)
  return { title, detail: same ? t('expenseForm.summary.each', { amount: money(first.amount) }) : list(owed, nameOf, money), problem: false }
}

function list(amounts: readonly Amount[], nameOf: Say['nameOf'], money: Say['money']): string {
  return amounts.map(({ memberId, amount }) => `${nameOf(memberId)} ${money(amount)}`).join(' · ')
}

// The line under the split while the Usuario types: why it does not close yet,
// or that it does. `valueProblem` points at the first field that cannot be read,
// and `formerChange` at a former member whose Cuota would change.
export function splitStatus(
  {
    total,
    method,
    issue,
    unassigned,
    percentSum,
    valueProblem,
    formerChange,
  }: {
    total: number
    method: SplitMethod
    issue?: SplitIssue
    unassigned: number
    percentSum: number
    valueProblem?: string
    formerChange?: { memberId: string; from: number; to: number }
  },
  { t, nameOf, money }: Say,
): Status {
  if (total <= 0) return { tone: 'quiet', text: t('expenseForm.amountFirst') }
  if (valueProblem) return { tone: 'problem', text: valueProblem }
  switch (issue) {
    case 'noParticipants':
      return { tone: 'problem', text: t(nobodyKey[method]) }
    case 'noWeights':
      return { tone: 'problem', text: t('expenseForm.noWeights') }
    case 'negativeShare':
      return { tone: 'problem', text: t('expenseForm.negativeShare') }
    case 'invalidValue':
      return { tone: 'problem', text: t('expenseForm.invalidValue') }
    case 'percentageNot100':
      return percentSum < 10000
        ? { tone: 'problem', text: t('expenseForm.percentMissing', { value: formatPercent(10000 - percentSum) }) }
        : { tone: 'problem', text: t('expenseForm.percentOver', { value: formatPercent(percentSum - 10000) }) }
  }
  if (unassigned > 0) return { tone: 'problem', text: t('expenseForm.missing', { amount: money(unassigned) }) }
  if (unassigned < 0) return { tone: 'problem', text: t('expenseForm.over', { amount: money(-unassigned) }) }
  if (formerChange) {
    const { memberId, from, to } = formerChange
    return { tone: 'problem', text: t('expenseForm.formerShareChanges', { name: nameOf(memberId), from: money(from), to: money(to) }) }
  }
  return { tone: 'even', text: t('expenseForm.closes', { total: money(total) }) }
}

// With nobody in the split, each method says what makes someone take part.
const nobodyKey = {
  equal: 'expenseForm.noParticipants',
  adjustment: 'expenseForm.noParticipants',
  exact: 'expenseForm.noParticipantsExact',
  percentage: 'expenseForm.noParticipantsPercentage',
  weights: 'expenseForm.noWeights',
} as const satisfies Record<SplitMethod, string>

// The line under the payers when several paid. `paidProblem` points at the first
// field that cannot be read.
export function payerStatus({ left, paidProblem }: { left: number; paidProblem?: string }, { t, money }: Say): Status {
  if (paidProblem) return { tone: 'problem', text: paidProblem }
  if (left > 0) return { tone: 'problem', text: t('expenseForm.payersLeft', { amount: money(left) }) }
  if (left < 0) return { tone: 'problem', text: t('expenseForm.payersOver', { amount: money(-left) }) }
  return { tone: 'even', text: t('expenseForm.payersOk') }
}
