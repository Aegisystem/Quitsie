import { describe, expect, it } from 'vitest'
import { splitExpense } from './split'

describe('equal', () => {
  it('splits the total among the chosen Participants only', () => {
    expect(splitExpense(9000, { method: 'equal', participantIds: ['ana', 'beto', 'carla'] })).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 3000, splitValue: null },
        { participantId: 'beto', amount: 3000, splitValue: null },
        { participantId: 'carla', amount: 3000, splitValue: null },
      ],
    })
  })

  it('gives the leftover minor units to the lowest ids, whatever the order on screen', () => {
    expect(splitExpense(100, { method: 'equal', participantIds: ['carla', 'ana', 'beto'] })).toEqual({
      ok: true,
      value: [
        { participantId: 'carla', amount: 33, splitValue: null },
        { participantId: 'ana', amount: 34, splitValue: null },
        { participantId: 'beto', amount: 33, splitValue: null },
      ],
    })
    expect(splitExpense(200, { method: 'equal', participantIds: ['carla', 'beto', 'ana'] })).toEqual({
      ok: true,
      value: [
        { participantId: 'carla', amount: 66, splitValue: null },
        { participantId: 'beto', amount: 67, splitValue: null },
        { participantId: 'ana', amount: 67, splitValue: null },
      ],
    })
  })

  it('leaves out whoever ends up owing nothing, since that is not an OwedShare', () => {
    expect(splitExpense(2, { method: 'equal', participantIds: ['ana', 'beto', 'carla'] })).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 1, splitValue: null },
        { participantId: 'beto', amount: 1, splitValue: null },
      ],
    })
  })
})

describe('any SplitMethod', () => {
  it('needs a total greater than zero', () => {
    for (const total of [0, -100]) {
      expect(splitExpense(total, { method: 'equal', participantIds: ['ana'] })).toEqual({
        ok: false,
        error: { code: 'total-not-positive' },
      })
    }
  })

  it('needs at least one Participant', () => {
    expect(splitExpense(100, { method: 'equal', participantIds: [] })).toEqual({
      ok: false,
      error: { code: 'no-participants' },
    })
  })

  it('rejects a Participant listed twice', () => {
    expect(splitExpense(100, { method: 'equal', participantIds: ['ana', 'beto', 'ana'] })).toEqual({
      ok: false,
      error: { code: 'duplicate-participant', participantId: 'ana' },
    })
  })

  it('throws when the total is not an integer amount in minor units', () => {
    expect(() => splitExpense(10.5, { method: 'equal', participantIds: ['ana'] })).toThrow()
  })
})

describe('exact', () => {
  it('keeps the amounts as typed when they add up to the total', () => {
    const split = {
      method: 'exact',
      amounts: [
        { participantId: 'ana', value: 2550 },
        { participantId: 'beto', value: 7450 },
      ],
    } as const
    expect(splitExpense(10000, split)).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 2550, splitValue: null },
        { participantId: 'beto', amount: 7450, splitValue: null },
      ],
    })
  })

  it('says by how much the amounts miss the total', () => {
    const amounts = [
      { participantId: 'ana', value: 2550 },
      { participantId: 'beto', value: 7000 },
    ]
    expect(splitExpense(10000, { method: 'exact', amounts })).toEqual({
      ok: false,
      error: { code: 'sum-mismatch', difference: -450 },
    })
    expect(splitExpense(9000, { method: 'exact', amounts })).toEqual({
      ok: false,
      error: { code: 'sum-mismatch', difference: 550 },
    })
  })

  it('rejects a negative amount', () => {
    const amounts = [
      { participantId: 'ana', value: 11000 },
      { participantId: 'beto', value: -1000 },
    ]
    expect(splitExpense(10000, { method: 'exact', amounts })).toEqual({
      ok: false,
      error: { code: 'invalid-value', participantId: 'beto' },
    })
  })

  it('leaves out a Participant whose amount is zero', () => {
    const amounts = [
      { participantId: 'ana', value: 10000 },
      { participantId: 'beto', value: 0 },
    ]
    expect(splitExpense(10000, { method: 'exact', amounts })).toEqual({
      ok: true,
      value: [{ participantId: 'ana', amount: 10000, splitValue: null }],
    })
  })
})

describe('percentage', () => {
  it('takes each percentage of the total and keeps it, in hundredths, to re-edit', () => {
    const percentages = [
      { participantId: 'ana', value: 5000 },
      { participantId: 'beto', value: 2500 },
      { participantId: 'carla', value: 2500 },
    ]
    expect(splitExpense(1000, { method: 'percentage', percentages })).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 500, splitValue: 5000 },
        { participantId: 'beto', amount: 250, splitValue: 2500 },
        { participantId: 'carla', amount: 250, splitValue: 2500 },
      ],
    })
  })

  it('gives the leftover to the largest remainder before looking at ids', () => {
    // 33.33 %, 33.33 % and 33.34 % of 10.00 are 3.333, 3.333 and 3.334.
    const percentages = [
      { participantId: 'ana', value: 3333 },
      { participantId: 'beto', value: 3333 },
      { participantId: 'carla', value: 3334 },
    ]
    expect(splitExpense(1000, { method: 'percentage', percentages })).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 333, splitValue: 3333 },
        { participantId: 'beto', amount: 333, splitValue: 3333 },
        { participantId: 'carla', amount: 334, splitValue: 3334 },
      ],
    })
  })

  it('needs the percentages to add up to exactly 100 %', () => {
    const percentages = [
      { participantId: 'ana', value: 5000 },
      { participantId: 'beto', value: 4000 },
    ]
    expect(splitExpense(1000, { method: 'percentage', percentages })).toEqual({
      ok: false,
      error: { code: 'sum-mismatch', difference: -1000 },
    })
  })

  it('rejects negative percentages and fractions of a hundredth', () => {
    for (const value of [-1000, 0.5]) {
      const percentages = [
        { participantId: 'ana', value: 10000 },
        { participantId: 'beto', value },
      ]
      expect(splitExpense(1000, { method: 'percentage', percentages })).toEqual({
        ok: false,
        error: { code: 'invalid-value', participantId: 'beto' },
      })
    }
  })
})
