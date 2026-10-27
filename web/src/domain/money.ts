import { type CurrencyCode, minorUnitDigits } from './currencies'
import { fail, ok, type Result } from './result'

// Amounts are integers in the currency's minor unit (docs/arquitectura.md):
// 1050 is 10.50 USD and 1050 JPY. They never go through floating point.
// No amount goes over 10^15 minor units, so that adding up Balances never
// leaves the safe integers (docs/reglas-de-negocio.md).
export const maxAmount = 10 ** 15

export function assertAmount(amount: number): void {
  if (!Number.isSafeInteger(amount)) {
    throw new Error(`${amount} is not an integer amount in minor units`)
  }
}

// '10.50' for 1050 with 2 decimals: the exact decimal, without going through a float.
function toDecimalString(amount: number, digits: number): string {
  const sign = amount < 0 ? '-' : ''
  const units = String(Math.abs(amount)).padStart(digits + 1, '0')
  if (digits === 0) return sign + units
  return `${sign}${units.slice(0, -digits)}.${units.slice(-digits)}`
}

export function formatAmount(amount: number, currency: CurrencyCode, locale: string): string {
  assertAmount(amount)
  const digits = minorUnitDigits(currency)
  const formatter = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency,
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
  // Intl formats a decimal string exactly (ES2023), but the ES2022 types in
  // tsconfig.app.json only declare numbers.
  return formatter.format(toDecimalString(amount, digits) as unknown as number)
}

export type ParseError = 'empty' | 'invalid' | 'too-many-decimals' | 'too-large'

function separators(locale: string): { group: string; decimal: string } {
  const parts = new Intl.NumberFormat(locale).formatToParts(1234567.8)
  const find = (type: Intl.NumberFormatPartTypes) => parts.find((part) => part.type === type)?.value
  return { group: find('group') ?? '', decimal: find('decimal') ?? '.' }
}

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// Reads a decimal number typed in the locale's format as an integer count of
// 10^-digits: ('10,50', 'es-CO', 2) is 1050.
function parseDecimal(input: string, locale: string, digits: number): Result<number, ParseError> {
  const { group, decimal } = separators(locale)
  let text = input.trim()
  if (text === '') return fail('empty')
  // fr-FR groups with a narrow no-break space, which nobody can type.
  if (/^\s$/.test(group)) text = text.replace(/\s/g, group)
  // Phones and habit put a point in amounts everywhere, so a lone point is the
  // decimal separator unless exactly three digits follow it: '10.50' is 10,50 in
  // es-CO, where '.' groups thousands, but '1.500' is still 1500.
  const point = /^[-−]?\d*\.(\d*)$/.exec(text)
  if (decimal !== '.' && point && point[1]?.length !== 3) text = text.replace('.', decimal)

  // Otherwise thousands separators must sit every three digits, so that a typo
  // never silently multiplies an amount. The sign may be a hyphen or the minus
  // sign (U+2212) that Intl writes in some locales.
  const g = escapeRegExp(group)
  const d = escapeRegExp(decimal)
  const match = new RegExp(`^([-−]?)(\\d{1,3}(?:${g}\\d{3})+|\\d*)(?:${d}(\\d*))?$`).exec(text)
  const whole = match?.[2]?.replaceAll(group, '') ?? ''
  const fraction = match?.[3] ?? ''
  if (!match || whole + fraction === '') return fail('invalid')
  if (fraction.length > digits) return fail('too-many-decimals')

  const units = BigInt(whole || '0') * 10n ** BigInt(digits) + BigInt(fraction.padEnd(digits, '0') || '0')
  if (units > BigInt(maxAmount)) return fail('too-large')
  const value = Number(units)
  return ok(match[1] && value !== 0 ? -value : value)
}

// Reads what the Usuario typed ('10,50' in es-CO, '10.50' in en-US) as an
// integer number of minor units.
export function parseAmount(
  input: string,
  currency: CurrencyCode,
  locale: string,
): Result<number, ParseError> {
  return parseDecimal(input, locale, minorUnitDigits(currency))
}

// Reads a percentage for the 'percentage' SplitMethod in hundredths, the unit in
// which it is stored: '33,33' is 3333 and 100 % is 10000.
export function parsePercentage(input: string, locale: string): Result<number, ParseError> {
  return parseDecimal(input, locale, 2)
}
