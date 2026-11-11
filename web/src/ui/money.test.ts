import { describe, expect, it } from 'vitest'
import { tallyUnit } from './money'

describe('the unit of a token', () => {
  it('is 1, 2 or 5 × 10ⁿ so the largest amount needs at most ten tokens', () => {
    expect(tallyUnit(6500, 'USD')).toBe(1000)
    expect(tallyUnit(49000, 'MXN')).toBe(5000)
    expect(tallyUnit(150000, 'USD')).toBe(20000)
  })

  it('is never less than one whole unit of the currency', () => {
    expect(tallyUnit(300, 'USD')).toBe(100)
    expect(tallyUnit(0, 'USD')).toBe(100)
  })

  it('is never less than 5.000 pesos in COP', () => {
    expect(tallyUnit(900000, 'COP')).toBe(500000)
    expect(tallyUnit(300000, 'COP')).toBe(500000)
    expect(tallyUnit(0, 'COP')).toBe(500000)
  })

  it('grows past 5.000 pesos when the amount needs it', () => {
    expect(tallyUnit(9000000, 'COP')).toBe(1000000)
    expect(tallyUnit(30000000, 'COP')).toBe(5000000)
  })
})
