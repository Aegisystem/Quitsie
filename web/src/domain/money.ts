import { type CurrencyCode, minorUnitDigits } from './currencies'
import { fail, ok, type Result } from './result'

// Amounts are integers in the currency's minor unit (docs/arquitectura.md):
// 1050 is 10.50 USD and 1050 JPY. They never go through floating point.
// No record's amount goes over 10^15 minor units. That bounds one record, not
// a Balance, which adds up many: the Balances have their own limit, maxBalance
// in balances.ts (docs/reglas-de-negocio.md).
const maxAmount = 10 ** 15

export function assertAmount(amount: number): void {
  if (!Number.isSafeInteger(amount)) {
    throw new Error(`${amount} is not an integer amount in minor units`)
  }
}

// The amount to format, as a bigint. A number must be an integer. Past the
// safe integers it is already rounded, and it is shown as it is rather than
// failing the screen: a provisional Balance can go past maxBalance until the
// server refuses the record, and a form adds up whatever was typed.
function unitsOf(amount: number | bigint): bigint {
  if (typeof amount === 'bigint') return amount
  if (!Number.isInteger(amount)) throw new Error(`${amount} is not an integer amount in minor units`)
  return BigInt(amount)
}

// '10.50' for 1050 with 2 decimals: the exact decimal, without going through a float.
function toDecimalString(amount: bigint, digits: number): string {
  const sign = amount < 0n ? '-' : ''
  const units = String(amount < 0n ? -amount : amount).padStart(digits + 1, '0')
  if (digits === 0) return sign + units
  return `${sign}${units.slice(0, -digits)}.${units.slice(-digits)}`
}

// Browsers accept useGrouping: 'always' (ES2023), but the ES2022 types in
// tsconfig.app.json only know booleans.
type NumberFormatOptions = Omit<Intl.NumberFormatOptions, 'useGrouping'> & { useGrouping?: boolean | 'always' }

const formatters = new Map<string, Intl.NumberFormat>()

// Every amount is written with Latin digits, the ones the Usuario types and
// parseAmount reads, in the separators of their region. Formatters are cached:
// a listing formats hundreds of amounts.
function numberFormat(locale: string, options: NumberFormatOptions = {}): Intl.NumberFormat {
  const key = `${locale}|${JSON.stringify(options)}`
  let formatter = formatters.get(key)
  if (!formatter) {
    formatter = new Intl.NumberFormat(locale, { ...options, numberingSystem: 'latn' } as Intl.NumberFormatOptions)
    formatters.set(key, formatter)
  }
  return formatter
}

export interface FormatOptions {
  // Listings drop the decimals when they are all zero ('45 €', but '45,50 €');
  // forms always show them.
  hideZeroDecimals?: boolean
  // '+' on positive amounts, for Balances in someone's favour. Zero stays unsigned.
  signed?: boolean
}

// A bigint formats exactly, however large: a total across ledgers can go past
// the safe integers.
export function formatAmount(
  amount: number | bigint,
  currency: CurrencyCode,
  locale: string,
  options: FormatOptions = {},
): string {
  let units = unitsOf(amount)
  let digits = minorUnitDigits(currency)
  const scale = 10n ** BigInt(digits)
  if (options.hideZeroDecimals && units % scale === 0n) {
    units /= scale
    digits = 0
  }
  const formatter = numberFormat(locale, {
    style: 'currency',
    currency,
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
    signDisplay: options.signed ? 'exceptZero' : 'auto',
    // Spanish CLDR only groups from five digits ('7589,67 €'); amounts read
    // better grouped every time ('7.589,67 €'), as in es-CO and es-MX.
    useGrouping: 'always',
  })
  // Intl formats a decimal string exactly (ES2023), but the ES2022 types only
  // declare numbers.
  return formatter.format(toDecimalString(units, digits) as unknown as number)
}

// The amount as it shows in a form field: every decimal and the region's
// thousands separators, without symbol ('1.234,56' in es-CO). parseAmount reads
// it back, and formatAmountTyping keeps it that way while the Usuario edits it.
export function formatAmountInput(amount: number, currency: CurrencyCode, locale: string): string {
  const units = unitsOf(amount)
  const { group, decimal } = separators(locale)
  const [whole = '0', fraction] = toDecimalString(units < 0n ? -units : units, minorUnitDigits(currency)).split('.')
  return (units < 0n ? '-' : '') + groupDigits(whole, group) + (fraction === undefined ? '' : decimal + fraction)
}

// The whole part grouped every three digits, the only grouping parseAmount
// accepts: '1234567' is '1.234.567' in es-CO (Intl would write '12,34,567' in hi-IN).
function groupDigits(whole: string, group: string): string {
  let grouped = ''
  for (let end = whole.length; end > 0; end -= 3) {
    const chunk = whole.slice(Math.max(0, end - 3), end)
    grouped = grouped ? chunk + group + grouped : chunk
  }
  return grouped
}

// One change to an amount field: its new text, where the caret ended up, and
// what the browser says caused it (InputEvent.inputType and .data).
export interface AmountEdit {
  text: string
  caret: number
  inputType?: string
  data?: string | null
}

// Rewrites an amount field while the Usuario types: the whole part takes the
// region's thousands separators (typing 50000 in es-CO shows '50.000') and the
// caret stays among the same digits. The separators it puts are its own, so
// they are dropped and put again on every change. A decimal separator is the
// region's one, or a point the Usuario has just typed or pasted where the
// region groups with points, which parseAmount reads as decimal the same way.
// Everything else that is not a digit goes; what is left may still be wrong
// (too many decimals), and parseAmount says why.
export function formatAmountTyping(
  edit: AmountEdit,
  currency: CurrencyCode,
  locale: string,
  options: { allowNegative?: boolean } = {},
): { text: string; caret: number } {
  const { text, caret } = edit
  const decimals = minorUnitDigits(currency)
  const { group, decimal } = separators(locale)

  let decimalAt = decimals > 0 ? text.indexOf(decimal) : -1
  if (decimalAt === -1 && decimals > 0 && decimal !== '.') {
    if (edit.inputType === 'insertText' && edit.data === '.' && text[caret - 1] === '.') decimalAt = caret - 1
    else if (edit.inputType === 'insertFromPaste' && /^[-−]?\d*\.(\d{0,2}|\d{4,})$/.test(text)) decimalAt = text.indexOf('.')
  }

  // Each character of the result, and the index of the character it comes
  // from, so that the caret can follow it.
  type Piece = { char: string; from?: number }
  const sign: Piece[] = []
  let whole: Piece[] = []
  const fraction: Piece[] = []
  for (let i = 0; i < text.length; i++) {
    const char = text[i] ?? ''
    const afterDecimal = decimalAt !== -1 && i > decimalAt
    if (/\d/.test(char)) (afterDecimal ? fraction : whole).push({ char, from: i })
    else if (/[-−]/.test(char) && options.allowNegative && !sign.length && !whole.length && !afterDecimal) sign.push({ char: '-', from: i })
  }
  // '007' is 7; the zero of '0,5' stays, and ',5' gets one.
  while (whole.length > 1 && whole[0]?.char === '0') whole.shift()
  if (!whole.length && decimalAt !== -1) whole = [{ char: '0' }]

  const pieces: Piece[] = [...sign]
  whole.forEach((piece, i) => {
    if (i > 0 && (whole.length - i) % 3 === 0) pieces.push({ char: group })
    pieces.push(piece)
  })
  if (decimalAt !== -1) pieces.push({ char: decimal, from: decimalAt }, ...fraction)

  // The caret goes right after the last character that was before it.
  let position = 0
  pieces.forEach((piece, i) => {
    if (piece.from !== undefined && piece.from < caret) position = i + 1
  })
  return { text: pieces.map((piece) => piece.char).join(''), caret: position }
}

export type ParseError = 'empty' | 'invalid' | 'too-many-decimals' | 'too-large'

function separators(locale: string): { group: string; decimal: string } {
  const parts = numberFormat(locale).formatToParts(1234567.8)
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
  // Right-to-left locales write a direction mark before the sign ('\u200e-5.25').
  let text = input.replace(/[\u200e\u200f\u061c]/g, '').trim()
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
