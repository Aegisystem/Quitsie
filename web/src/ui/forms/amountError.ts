import type { TFunction } from 'i18next'
import { type Balance, type CurrencyCode, minorUnitDigits, overMaxBalance, type ParseError, type Records, type Result } from '../../domain'
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

// Why a record cannot be saved because it would take someone's Balance past
// what Quitsie can hold (docs/reglas-de-negocio.md), if it would. `balances`
// are the ledger's provisional ones; `removed` is what an edit replaces.
export function balanceError(
  balances: readonly Balance[],
  change: { added: Records; removed?: Records },
  people: { me: string; nameOf: (memberId: string) => string },
  t: TFunction,
): string | undefined {
  const over = overMaxBalance(balances, change.added, change.removed)
  if (over === undefined) return undefined
  return over === people.me ? t('expenseForm.yourBalanceTooLarge') : t('expenseForm.balanceTooLarge', { name: people.nameOf(over) })
}
