import { describe, expect, it } from 'vitest'
import { currencyChoices, isCurrency, minorUnitDigits, offeredCurrencies } from './currencies'

describe('minorUnitDigits', () => {
  it.each([
    ['JPY', 0],
    ['CLP', 0],
    ['USD', 2],
    ['COP', 2],
    ['EUR', 2],
    ['BHD', 3],
    ['KWD', 3],
    ['CLF', 4],
  ])('%s has %i decimals', (currency, digits) => {
    expect(minorUnitDigits(currency)).toBe(digits)
  })

  it('rejects a code that is not a current ISO 4217 currency', () => {
    expect(() => minorUnitDigits('HRK')).toThrow('HRK')
  })
})

describe('isCurrency', () => {
  it('accepts current ISO 4217 codes', () => {
    expect(isCurrency('COP')).toBe(true)
    expect(isCurrency('XCG')).toBe(true)
  })

  it('rejects withdrawn codes, codes without a minor unit and lower case', () => {
    expect(isCurrency('ANG')).toBe(false)
    expect(isCurrency('XAU')).toBe(false)
    expect(isCurrency('XXX')).toBe(false)
    expect(isCurrency('usd')).toBe(false)
  })
})

describe('offeredCurrencies', () => {
  it('offers every current currency in alphabetical order', () => {
    const offered = offeredCurrencies()
    expect(offered).toContain('COP')
    expect(offered).toContain('XCG')
    expect(offered).toEqual([...offered].sort())
    expect(offered.every(isCurrency)).toBe(true)
  })

  it('leaves out the ISO 4217 fund codes, which nobody pays with, though they stay valid', () => {
    const funds = ['BOV', 'CHE', 'CHW', 'CLF', 'COU', 'MXV', 'USN', 'UYI', 'UYW', 'XAD']
    expect(offeredCurrencies().filter((code) => funds.includes(code))).toEqual([])
    expect(offeredCurrencies()).toHaveLength(165 - funds.length)
    expect(funds.every(isCurrency)).toBe(true)
  })
})

describe('currencyChoices', () => {
  it('puts the Preferred currency first, then the rest in alphabetical order', () => {
    const { first, rest } = currencyChoices('MXN')
    expect(first).toEqual(['MXN'])
    expect(rest).toEqual(offeredCurrencies().filter((code) => code !== 'MXN'))
  })

  it('follows the Preferred currency with the most used ones, most used first', () => {
    const used = ['EUR', 'USD', 'EUR', 'COP', 'EUR', 'USD']
    expect(currencyChoices('MXN', used).first).toEqual(['MXN', 'EUR', 'USD', 'COP'])
  })

  it('breaks a tie between used currencies by code', () => {
    expect(currencyChoices('MXN', ['USD', 'EUR']).first).toEqual(['MXN', 'EUR', 'USD'])
  })

  it('does not repeat the Preferred currency however much it is used', () => {
    expect(currencyChoices('USD', ['USD', 'USD', 'EUR']).first).toEqual(['USD', 'EUR'])
  })

  it('keeps every currency exactly once', () => {
    const { first, rest } = currencyChoices('MXN', ['EUR', 'USD', 'EUR'])
    expect([...first, ...rest].sort()).toEqual(offeredCurrencies())
  })

  it('brings up only the most used ones, so the short list stays short', () => {
    const used = ['USD', 'USD', 'USD', 'USD', 'EUR', 'EUR', 'EUR', 'GBP', 'GBP', 'JPY', 'CHF']
    expect(currencyChoices('MXN', used).first).toEqual(['MXN', 'USD', 'EUR', 'GBP', 'CHF'])
  })

  it('never offers a fund, even if the User has it', () => {
    const { first, rest } = currencyChoices('CLF', ['CLF', 'UYI', 'EUR'])
    expect(first).toEqual(['EUR'])
    expect(rest).not.toContain('CLF')
    expect(rest).not.toContain('UYI')
  })
})
