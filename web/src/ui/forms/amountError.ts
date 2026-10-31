import type { TFunction } from 'i18next'
import { type CurrencyCode, minorUnitDigits, type ParseError, type Result } from '../../domain'
import { formatAmountInput } from '../money'

// Why what was typed in an amount field cannot be saved, if anything.
export function amountError(read: Result<number, ParseError>, currency: CurrencyCode, t: TFunction): string | undefined {
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
