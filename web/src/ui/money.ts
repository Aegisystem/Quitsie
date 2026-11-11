import {
  formatAmount,
  formatAmountInput as formatAmountInputIn,
  minorUnitDigits,
  type ParseError,
  parseAmount as parseAmountIn,
  parsePercentage,
  type Result,
} from '../domain'

// Amounts travel as integers in the currency's minor unit (docs/arquitectura.md)
// and the domain formats and reads them. Numbers follow the Usuario's region,
// whatever the language of the interface: 7,589.67 in es-MX, 7.589,67 in es-CO.
export function regionLocale(): string {
  return (typeof navigator !== 'undefined' && navigator.language) || 'es'
}

// Listings hide the decimals when they are zero ('45 €'); forms pass
// `trim: false` to always show them ('45,00 €').
export function formatMoney(minor: number | bigint, currency: string, opts: { signed?: boolean; trim?: boolean } = {}): string {
  return formatAmount(minor, currency, regionLocale(), { signed: opts.signed, hideZeroDecimals: opts.trim ?? true })
}

// The number for an input field, as the Usuario would type it; empty for zero.
export function formatAmountInput(minor: number, currency: string): string {
  return minor === 0 ? '' : formatAmountInputIn(minor, currency, regionLocale())
}

// Zero with every decimal ('0,00' or '0.00'), which shows the region's decimal
// separator before anything is typed.
export function amountPlaceholder(currency: string): string {
  return formatAmountInputIn(0, currency, regionLocale())
}

// What the Usuario typed, in minor units, or why it is not an amount.
export function readAmount(text: string, currency: string): Result<number, ParseError> {
  return parseAmountIn(text, currency, regionLocale())
}

// Percentages are stored in hundredths (10000 = 100 %) and written, like
// amounts, with Latin digits in the region's separators.
export function formatPercent(hundredths: number): string {
  return new Intl.NumberFormat(regionLocale(), { maximumFractionDigits: 2, numberingSystem: 'latn' }).format(hundredths / 100) + ' %'
}

export function formatPercentInput(hundredths: number): string {
  const options = { maximumFractionDigits: 2, useGrouping: false, numberingSystem: 'latn' }
  return new Intl.NumberFormat(regionLocale(), options).format(hundredths / 100)
}

// What the Usuario typed as a percentage, in hundredths, or why it is not one.
export function readPercent(text: string): Result<number, ParseError> {
  return parsePercentage(text.replace('%', ''), regionLocale())
}

// The smallest a token may stand for, in the major unit, where one unit is
// too little to count: 1.000 pesos buy almost nothing, 5.000 are a coin's worth.
const smallestTallyUnit: Readonly<Record<string, number>> = { COP: 5000 }

// The unit one token stands for in a tally: a round number (1, 2 or 5 × 10ⁿ in
// the major unit) so that the largest amount needs at most `maxTokens` tokens,
// and never below the currency's smallest.
export function tallyUnit(maxMinor: number, currency: string, maxTokens = 10): number {
  const scale = 10 ** minorUnitDigits(currency)
  const smallest = (smallestTallyUnit[currency] ?? 1) * scale
  return Math.max(smallest, roundTallyUnit(Math.abs(maxMinor) / scale, scale, maxTokens))
}

function roundTallyUnit(major: number, scale: number, maxTokens: number): number {
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
