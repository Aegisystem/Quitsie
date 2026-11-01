import { describe, expect, it } from 'vitest'
import { type Band, bandsOf, emptyingMs, tokensOf, withSettling } from './tally'

describe('a row of tokens', () => {
  it('has a whole token per unit and cuts the last one', () => {
    expect(tokensOf(6500, 1000)).toEqual([1, 1, 1, 1, 1, 1, 0.5])
    expect(tokensOf(-2500, 500)).toEqual([1, 1, 1, 1, 1])
  })

  it('leaves out a part too small to see', () => {
    expect(tokensOf(1040, 1000)).toEqual([1])
    expect(tokensOf(0, 1000)).toEqual([])
  })

  it('takes as long to empty as its tokens take to leave one after another', () => {
    expect(emptyingMs(0)).toBe(0)
    expect(emptyingMs(1)).toBe(500)
    expect(emptyingMs(5)).toBe(660)
  })
})

describe('the bands of Balance', () => {
  const band = (currency: string, amount: number, unit: number): Band => ({ currency, amount, unit })
  const usd = band('USD', 6500, 1000)
  const mxn = band('MXN', 49000, 5000)

  it('have one band per currency, in order, each with its own unit', () => {
    expect(bandsOf(new Map([['USD', 6500], ['MXN', 49000]]))).toEqual([usd, mxn])
  })

  it('show the new ones as they are when no currency reached zero', () => {
    expect(withSettling([usd, mxn], [band('USD', 2500, 500), mxn])).toEqual({ bands: [band('USD', 2500, 500), mxn], settling: [] })
  })

  it('keep a currency that reached zero in its place, at zero, with the unit its tokens had', () => {
    const cop = band('COP', 3000000, 200000)
    const { bands, settling } = withSettling([cop, mxn, usd], [mxn, usd])
    expect(bands).toEqual([band('COP', 0, 200000), mxn, usd])
    expect(settling).toEqual([cop])
  })

  it('keep the last band at zero when everything is settled', () => {
    expect(withSettling([usd], [])).toEqual({ bands: [band('USD', 0, 1000)], settling: [usd] })
  })

  it('put a new currency after those already shown', () => {
    const eur = band('EUR', 1000, 100)
    expect(withSettling([usd, mxn], [eur, mxn]).bands).toEqual([band('USD', 0, 1000), mxn, eur])
  })
})
