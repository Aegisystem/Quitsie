import { describe, expect, it } from 'vitest'
import { formatAmount, formatAmountInput, parseAmount, parsePercentage } from './money'

// CLDR separates the currency symbol with a no-break space in these locales.
const nbsp = ' '

describe('formatAmount', () => {
  it('shows the minor unit as decimals in the locale’s format', () => {
    expect(formatAmount(1050, 'USD', 'en-US')).toBe('$10.50')
    expect(formatAmount(123456, 'COP', 'es-CO')).toBe(`$${nbsp}1.234,56`)
    expect(formatAmount(1500, 'JPY', 'en-US')).toBe('¥1,500')
    expect(formatAmount(1234, 'KWD', 'en-US')).toBe(`KWD${nbsp}1.234`)
  })

  it('follows the region’s separators and groups every amount from four digits', () => {
    expect(formatAmount(758967, 'MXN', 'es-MX')).toBe('$7,589.67')
    expect(formatAmount(758967, 'COP', 'es-CO')).toBe(`$${nbsp}7.589,67`)
    expect(formatAmount(758967, 'EUR', 'es-ES')).toBe(`7.589,67${nbsp}€`)
  })

  it('writes Latin digits in every region, the ones parseAmount reads', () => {
    expect(formatAmount(758967, 'EGP', 'ar-EG')).toMatch(/7,589\.67/)
  })

  it('always shows every decimal of the currency', () => {
    expect(formatAmount(1000, 'USD', 'en-US')).toBe('$10.00')
    expect(formatAmount(5, 'USD', 'en-US')).toBe('$0.05')
  })

  it('can hide the decimals when they are all zero, as listings do', () => {
    const listing = { hideZeroDecimals: true }
    expect(formatAmount(4500, 'EUR', 'es-ES', listing)).toBe(`45${nbsp}€`)
    expect(formatAmount(4550, 'EUR', 'es-ES', listing)).toBe(`45,50${nbsp}€`)
    expect(formatAmount(4505, 'EUR', 'es-ES', listing)).toBe(`45,05${nbsp}€`)
    expect(formatAmount(-100000, 'USD', 'en-US', listing)).toBe('-$1,000')
    expect(formatAmount(1500, 'JPY', 'en-US', listing)).toBe('¥1,500')
  })

  it('formats negative amounts, as in a Balance against someone', () => {
    expect(formatAmount(-1050, 'USD', 'en-US')).toBe('-$10.50')
  })

  it('can sign a Balance both ways, leaving zero unsigned', () => {
    const signed = { signed: true }
    expect(formatAmount(1050, 'USD', 'en-US', signed)).toBe('+$10.50')
    expect(formatAmount(-1050, 'USD', 'en-US', signed)).toBe('-$10.50')
    expect(formatAmount(0, 'USD', 'en-US', signed)).toBe('$0.00')
  })

  it('formats the largest safe amounts without floating-point rounding', () => {
    expect(formatAmount(9007199254740985, 'USD', 'en-US')).toBe('$90,071,992,547,409.85')
  })
})

describe('formatAmountInput', () => {
  it('writes an amount as the Usuario would type it: every decimal, no symbol or grouping', () => {
    expect(formatAmountInput(123456, 'COP', 'es-CO')).toBe('1234,56')
    expect(formatAmountInput(123456, 'MXN', 'es-MX')).toBe('1234.56')
    expect(formatAmountInput(4500, 'EUR', 'es-ES')).toBe('45,00')
    expect(formatAmountInput(1500, 'JPY', 'ja-JP')).toBe('1500')
  })

  it('is read back by parseAmount as the same amount in any region', () => {
    const locales = ['es-CO', 'es-MX', 'es-ES', 'en-US', 'fr-FR', 'de-CH', 'sv-SE', 'ar-EG', 'he-IL', 'hi-IN']
    for (const locale of locales) {
      for (const currency of ['USD', 'JPY', 'KWD', 'CLF']) {
        for (const amount of [0, 5, 999, 1000, 1500, 123456, -525, 10 ** 15, -(10 ** 15)]) {
          const typed = formatAmountInput(amount, currency, locale)
          expect(parseAmount(typed, currency, locale), `${typed} in ${locale}`).toEqual({ ok: true, value: amount })
        }
      }
    }
  })
})

describe('parseAmount', () => {
  it('reads the decimal separator of the locale', () => {
    expect(parseAmount('10,50', 'COP', 'es-CO')).toEqual({ ok: true, value: 1050 })
    expect(parseAmount('10.50', 'USD', 'en-US')).toEqual({ ok: true, value: 1050 })
  })

  it('reads whole amounts and missing decimals', () => {
    expect(parseAmount('10', 'USD', 'en-US')).toEqual({ ok: true, value: 1000 })
    expect(parseAmount('10.5', 'USD', 'en-US')).toEqual({ ok: true, value: 1050 })
    expect(parseAmount('1500', 'JPY', 'ja-JP')).toEqual({ ok: true, value: 1500 })
    expect(parseAmount('1.234', 'KWD', 'en-US')).toEqual({ ok: true, value: 1234 })
  })

  it('accepts the locale’s thousands separators, typed with an ordinary space where it is a space', () => {
    expect(parseAmount('1.234,56', 'COP', 'es-CO')).toEqual({ ok: true, value: 123456 })
    expect(parseAmount('1.500', 'COP', 'es-CO')).toEqual({ ok: true, value: 150000 })
    expect(parseAmount('1,234,567.89', 'USD', 'en-US')).toEqual({ ok: true, value: 123456789 })
    expect(parseAmount('1 234,56', 'EUR', 'fr-FR')).toEqual({ ok: true, value: 123456 })
  })

  it('reads a point as decimal unless exactly three digits follow it, even where it groups thousands', () => {
    expect(parseAmount('10.50', 'COP', 'es-CO')).toEqual({ ok: true, value: 1050 })
    expect(parseAmount('10.5', 'COP', 'es-CO')).toEqual({ ok: true, value: 1050 })
    expect(parseAmount('10.', 'COP', 'es-CO')).toEqual({ ok: true, value: 1000 })
    expect(parseAmount('-2.75', 'COP', 'es-CO')).toEqual({ ok: true, value: -275 })
    expect(parseAmount('10.50', 'EUR', 'fr-FR')).toEqual({ ok: true, value: 1050 })
    expect(parseAmount('1.500', 'COP', 'es-CO')).toEqual({ ok: true, value: 150000 })
    expect(parseAmount('1.2345', 'COP', 'es-CO')).toEqual({ ok: false, error: 'too-many-decimals' })
  })

  it('does not guess with a comma, nor with a point next to the locale’s decimal separator', () => {
    expect(parseAmount('10,50', 'USD', 'en-US')).toEqual({ ok: false, error: 'invalid' })
    expect(parseAmount('10,50', 'MXN', 'es-MX')).toEqual({ ok: false, error: 'invalid' })
    expect(parseAmount('1.23,5', 'COP', 'es-CO')).toEqual({ ok: false, error: 'invalid' })
    expect(parseAmount('1.234.56', 'COP', 'es-CO')).toEqual({ ok: false, error: 'invalid' })
  })

  it('rejects more decimals than the currency has', () => {
    expect(parseAmount('10,505', 'COP', 'es-CO')).toEqual({ ok: false, error: 'too-many-decimals' })
    expect(parseAmount('10.5', 'JPY', 'en-US')).toEqual({ ok: false, error: 'too-many-decimals' })
    expect(parseAmount('1.2345', 'KWD', 'en-US')).toEqual({ ok: false, error: 'too-many-decimals' })
  })

  it('tolerates spaces around the amount and a separator with nothing on one side', () => {
    expect(parseAmount(' 10,50 ', 'COP', 'es-CO')).toEqual({ ok: true, value: 1050 })
    expect(parseAmount('10,', 'COP', 'es-CO')).toEqual({ ok: true, value: 1000 })
    expect(parseAmount(',5', 'COP', 'es-CO')).toEqual({ ok: true, value: 50 })
  })

  it('reads a minus sign, for Ajustes that take something off', () => {
    expect(parseAmount('-5,25', 'COP', 'es-CO')).toEqual({ ok: true, value: -525 })
    expect(parseAmount('\u22125.25', 'USD', 'en-US')).toEqual({ ok: true, value: -525 })
  })

  it('tells an empty field apart from text that is not an amount', () => {
    expect(parseAmount('  ', 'USD', 'en-US')).toEqual({ ok: false, error: 'empty' })
    for (const input of ['abc', '$10', '1..2', ',', '-', '1-2', '1e3']) {
      expect(parseAmount(input, 'USD', 'en-US')).toEqual({ ok: false, error: 'invalid' })
    }
  })

  it('rejects more than 10^15 minor units, so that sums of Balances never overflow', () => {
    expect(parseAmount('10000000000000.00', 'USD', 'en-US')).toEqual({ ok: true, value: 10 ** 15 })
    expect(parseAmount('10000000000000.01', 'USD', 'en-US')).toEqual({ ok: false, error: 'too-large' })
    expect(parseAmount('-10000000000000.01', 'USD', 'en-US')).toEqual({ ok: false, error: 'too-large' })
    expect(parseAmount('1000000000000001', 'JPY', 'ja-JP')).toEqual({ ok: false, error: 'too-large' })
    expect(parseAmount('99999999999999999999999', 'USD', 'en-US')).toEqual({ ok: false, error: 'too-large' })
  })
})

describe('parsePercentage', () => {
  it('reads up to two decimals as hundredths of a percent', () => {
    expect(parsePercentage('33,33', 'es-CO')).toEqual({ ok: true, value: 3333 })
    expect(parsePercentage('100', 'en-US')).toEqual({ ok: true, value: 10000 })
    expect(parsePercentage('12.5', 'en-US')).toEqual({ ok: true, value: 1250 })
  })

  it('rejects a third decimal', () => {
    expect(parsePercentage('33,333', 'es-CO')).toEqual({ ok: false, error: 'too-many-decimals' })
  })
})
