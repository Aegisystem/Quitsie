import { describe, expect, it } from 'vitest'
import { formatAmount, parseAmount, parsePercentage } from './money'

// CLDR separates the currency symbol with a no-break space in these locales.
const nbsp = ' '

describe('formatAmount', () => {
  it('shows the minor unit as decimals in the locale’s format', () => {
    expect(formatAmount(1050, 'USD', 'en-US')).toBe('$10.50')
    expect(formatAmount(123456, 'COP', 'es-CO')).toBe(`$${nbsp}1.234,56`)
    expect(formatAmount(1500, 'JPY', 'en-US')).toBe('¥1,500')
    expect(formatAmount(1234, 'KWD', 'en-US')).toBe(`KWD${nbsp}1.234`)
  })

  it('always shows every decimal of the currency', () => {
    expect(formatAmount(1000, 'USD', 'en-US')).toBe('$10.00')
    expect(formatAmount(5, 'USD', 'en-US')).toBe('$0.05')
  })

  it('formats negative amounts, as in a Balance against someone', () => {
    expect(formatAmount(-1050, 'USD', 'en-US')).toBe('-$10.50')
  })

  it('formats the largest safe amounts without floating-point rounding', () => {
    expect(formatAmount(9007199254740985, 'USD', 'en-US')).toBe('$90,071,992,547,409.85')
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

  it('rejects the other locale’s decimal separator instead of guessing', () => {
    expect(parseAmount('10.50', 'COP', 'es-CO')).toEqual({ ok: false, error: 'invalid' })
    expect(parseAmount('10,50', 'USD', 'en-US')).toEqual({ ok: false, error: 'invalid' })
    expect(parseAmount('1.2345', 'COP', 'es-CO')).toEqual({ ok: false, error: 'invalid' })
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

  it('rejects amounts too large to keep exactly', () => {
    expect(parseAmount('90071992547409.91', 'USD', 'en-US')).toEqual({ ok: true, value: 9007199254740991 })
    expect(parseAmount('90071992547409.92', 'USD', 'en-US')).toEqual({ ok: false, error: 'too-large' })
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
