import { describe, expect, it } from 'vitest'
import { multiplePayers, singlePayer } from './paidShares'

describe('singlePayer', () => {
  it('makes the payer’s PaidShare the whole total', () => {
    expect(singlePayer(9000, 'ana')).toEqual({ ok: true, value: [{ participantId: 'ana', amount: 9000 }] })
  })

  it('needs a total greater than zero', () => {
    expect(singlePayer(0, 'ana')).toEqual({ ok: false, error: { code: 'total-not-positive' } })
  })
})

describe('multiplePayers', () => {
  it('keeps what each payer put in when it adds up to the total', () => {
    const amounts = [
      { participantId: 'ana', value: 5000 },
      { participantId: 'beto', value: 4000 },
    ]
    expect(multiplePayers(9000, amounts)).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 5000 },
        { participantId: 'beto', amount: 4000 },
      ],
    })
  })

  it('says by how much the PaidShares miss the total', () => {
    const amounts = [
      { participantId: 'ana', value: 5000 },
      { participantId: 'beto', value: 3000 },
    ]
    expect(multiplePayers(9000, amounts)).toEqual({ ok: false, error: { code: 'sum-mismatch', difference: -1000 } })
  })

  it('leaves out whoever put in nothing and rejects negative amounts', () => {
    expect(
      multiplePayers(9000, [
        { participantId: 'ana', value: 9000 },
        { participantId: 'beto', value: 0 },
      ]),
    ).toEqual({ ok: true, value: [{ participantId: 'ana', amount: 9000 }] })
    expect(
      multiplePayers(9000, [
        { participantId: 'ana', value: 10000 },
        { participantId: 'beto', value: -1000 },
      ]),
    ).toEqual({ ok: false, error: { code: 'invalid-value', participantId: 'beto' } })
  })

  it('rejects a payer listed twice', () => {
    const amounts = [
      { participantId: 'ana', value: 4500 },
      { participantId: 'ana', value: 4500 },
    ]
    expect(multiplePayers(9000, amounts)).toEqual({
      ok: false,
      error: { code: 'duplicate-participant', participantId: 'ana' },
    })
  })
})
