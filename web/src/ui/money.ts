import i18n from 'i18next'

// Amounts travel as integers in the currency's minor unit (docs/arquitectura.md).
// Intl knows how many decimals each ISO 4217 currency has.

const formatters = new Map<string, Intl.NumberFormat>()

// Browsers accept useGrouping: 'always' (ES2023); the ES2022 lib types used here
// only know booleans.
type FormatOptions = Omit<Intl.NumberFormatOptions, 'useGrouping'> & { useGrouping?: boolean | 'always' }

function formatter(currency: string, signed: boolean, whole = false): Intl.NumberFormat {
  const key = `${i18n.language}|${currency}|${signed}|${whole}`
  let f = formatters.get(key)
  if (!f) {
    const options: FormatOptions = {
      style: 'currency',
      currency,
      signDisplay: signed ? 'exceptZero' : 'auto',
      minimumFractionDigits: whole ? 0 : undefined,
      // Spanish CLDR only groups from five digits ("7589,67"); amounts read
      // better grouped every time ("7.589,67").
      useGrouping: 'always',
    }
    f = new Intl.NumberFormat(i18n.language, options as Intl.NumberFormatOptions)
    formatters.set(key, f)
  }
  return f
}

export function minorDigits(currency: string): number {
  return formatter(currency, false).resolvedOptions().maximumFractionDigits ?? 2
}

export function toMajor(minor: number, currency: string): number {
  return minor / 10 ** minorDigits(currency)
}

// `trim` drops ",00" from whole amounts, for round figures like a tally's unit.
export function formatMoney(minor: number, currency: string, opts: { signed?: boolean; trim?: boolean } = {}): string {
  const major = toMajor(minor, currency)
  const whole = (opts.trim ?? false) && Number.isInteger(major)
  return formatter(currency, opts.signed ?? false, whole).format(major)
}

// Plain number for an input field, without currency symbol or grouping.
export function formatAmountInput(minor: number, currency: string): string {
  if (minor === 0) return ''
  const digits = minorDigits(currency)
  return new Intl.NumberFormat(i18n.language, {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
    useGrouping: false,
  }).format(toMajor(minor, currency))
}

// Reads what someone typed ("84,20", "84.20", "1.200", "1 200,5") as minor units.
// Returns null when it is not a number.
export function parseAmount(text: string, currency: string): number | null {
  const digits = minorDigits(currency)
  const cleaned = text.replace(/[\s '’]/g, '')
  if (cleaned === '') return null
  if (!/^-?[\d.,]+$/.test(cleaned)) return null
  const negative = cleaned.startsWith('-')
  const body = negative ? cleaned.slice(1) : cleaned

  let integer = body
  let fraction = ''
  const lastSep = Math.max(body.lastIndexOf(','), body.lastIndexOf('.'))
  if (lastSep >= 0 && digits > 0) {
    const tail = body.slice(lastSep + 1)
    const sepCount = body.split(body[lastSep] ?? '').length - 1
    // A single separator followed by no more digits than the currency allows is the
    // decimal one ("84,20", "84.2"); otherwise it groups thousands ("1.200").
    if (sepCount === 1 && tail.length <= digits) {
      integer = body.slice(0, lastSep)
      fraction = tail
    }
  }
  integer = integer.replace(/[.,]/g, '')
  if (integer === '' && fraction === '') return null
  const value = Number(integer || '0') * 10 ** digits + Number(fraction.padEnd(digits, '0') || '0')
  if (!Number.isFinite(value)) return null
  return negative ? -value : value
}

// Percentages are stored in hundredths (10000 = 100 %).
export function formatPercent(hundredths: number): string {
  return new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 2 }).format(hundredths / 100) + ' %'
}

export function parsePercent(text: string): number | null {
  const cleaned = text.replace(/[\s%]/g, '').replace(',', '.')
  if (cleaned === '') return null
  const n = Number(cleaned)
  if (!Number.isFinite(n)) return null
  return Math.round(n * 100)
}

// The unit one token stands for in a tally: a round number (1, 2 or 5 × 10ⁿ in
// the major unit) so that the largest amount needs at most `maxTokens` tokens.
export function tallyUnit(maxMinor: number, currency: string, maxTokens = 10): number {
  const scale = 10 ** minorDigits(currency)
  const major = Math.abs(maxMinor) / scale
  if (major === 0) return scale
  // Never smaller than one major unit: half a token reads better than "each = 0.05".
  let magnitude = Math.max(1, 10 ** Math.floor(Math.log10(major / maxTokens)))
  for (;;) {
    for (const step of [1, 2, 5]) {
      const unit = step * magnitude
      if (major / unit <= maxTokens) return Math.max(1, Math.round(unit * scale))
    }
    magnitude *= 10
  }
}
