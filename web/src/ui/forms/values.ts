import type { TFunction } from 'i18next'
import type { SplitMethod } from '../../api/client'
import type { ParseError, Result } from '../../domain'
import { amountError } from './amountError'
import { formatPercentInput, readAmount, readPercent, regionLocale } from '../money'

// A field the Expense form has for each person: what they paid, or their value
// in the split method (an amount, a percentage, a weight or an adjustment).
export type ValueField = 'paid' | Exclude<SplitMethod, 'equal'>

// Why such a field cannot be used. `not-whole`: a weight with decimals.
// `negative`: an Aporte below zero, which would leave that person out.
export type ValueError = Exclude<ParseError, 'empty'> | 'not-whole' | 'negative'

// What a person's field holds. Empty is zero, which leaves them out on purpose;
// anything else that cannot be read is an error, never a silent zero.
export function readValue(text: string, field: ValueField, currency: string): Result<number, ValueError> {
  const typed = text.trim()
  if (typed === '') return { ok: true, value: 0 }
  switch (field) {
    case 'exact':
      return known(readAmount(typed, currency))
    case 'adjustment':
      // The empty field shows "+0", so a plus sign is welcome.
      return known(readAmount(typed.replace(/^\+/, ''), currency))
    case 'paid': {
      const read = known(readAmount(typed, currency))
      return read.ok && read.value < 0 ? { ok: false, error: 'negative' } : read
    }
    case 'percentage':
      return known(readPercent(typed))
    case 'weights':
      return readWeight(typed)
  }
}

// Only a lone sign, such as "+", reads as empty here: it is not a number yet.
function known(read: Result<number, ParseError>): Result<number, ValueError> {
  if (read.ok) return read
  return { ok: false, error: read.error === 'empty' ? 'invalid' : read.error }
}

// Weights are whole numbers: a proportion like 1,5:1 is written 3:2
// (docs/reglas-de-negocio.md). Zero leaves someone out of the split.
function readWeight(typed: string): Result<number, ValueError> {
  if (/^\d+$/.test(typed)) {
    const value = Number(typed)
    return Number.isSafeInteger(value) ? { ok: true, value } : { ok: false, error: 'too-large' }
  }
  return { ok: false, error: /^\d*[.,]\d+$/.test(typed) ? 'not-whole' : 'invalid' }
}

// What to write instead, said like the total says it for amounts.
export function valueError(error: ValueError, field: ValueField, currency: string, t: TFunction): string {
  switch (field) {
    case 'percentage':
      if (error === 'too-many-decimals') return t('expenseForm.percentDecimals')
      if (error === 'too-large') return t('expenseForm.percentTooLarge')
      return t('expenseForm.percentInvalid', { example: formatPercentInput(3333) })
    case 'weights':
      if (error === 'not-whole') return t('expenseForm.weightWhole', { example: new Intl.NumberFormat(regionLocale(), { numberingSystem: 'latn' }).format(1.5) })
      if (error === 'too-large') return t('expenseForm.weightTooLarge')
      return t('expenseForm.weightInvalid')
    default:
      if (error === 'negative') return t('expenseForm.paidNegative')
      return amountError({ ok: false, error: error === 'not-whole' ? 'invalid' : error }, currency, t) ?? ''
  }
}
