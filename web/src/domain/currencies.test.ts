import { describe, expect, it } from 'vitest'
import { isCurrency, minorUnitDigits } from './currencies'

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
