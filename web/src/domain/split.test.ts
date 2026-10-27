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

describe('weights', () => {
  it('splits in proportion to the weights and keeps each weight to re-edit', () => {
    const weights = [
      { participantId: 'ana', value: 2 },
      { participantId: 'beto', value: 1 },
      { participantId: 'carla', value: 1 },
    ]
    expect(splitExpense(1000, { method: 'weights', weights })).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 500, splitValue: 2 },
        { participantId: 'beto', amount: 250, splitValue: 1 },
        { participantId: 'carla', amount: 250, splitValue: 1 },
      ],
    })
  })

  it('gives the leftover to the largest remainder', () => {
    // 1:2 of 1.00 is 0.333… and 0.666…
    const weights = [
      { participantId: 'ana', value: 1 },
      { participantId: 'beto', value: 2 },
    ]
    expect(splitExpense(100, { method: 'weights', weights })).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 33, splitValue: 1 },
        { participantId: 'beto', amount: 67, splitValue: 2 },
      ],
    })
  })

  it('only accepts positive whole weights', () => {
    for (const value of [0, -1, 1.5]) {
      const weights = [
        { participantId: 'ana', value: 1 },
        { participantId: 'beto', value },
      ]
      expect(splitExpense(100, { method: 'weights', weights })).toEqual({
        ok: false,
        error: { code: 'invalid-value', participantId: 'beto' },
      })
    }
  })
})

describe('adjustment', () => {
  it('splits what is left after the adjustments equally and adds each one back', () => {
    // 90.00 where Ana had 15.00 more and Carla 6.00 less: 81.00 is split three ways.
    const adjustments = [
      { participantId: 'ana', value: 1500 },
      { participantId: 'beto', value: 0 },
      { participantId: 'carla', value: -600 },
    ]
    expect(splitExpense(9000, { method: 'adjustment', adjustments })).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 4200, splitValue: 1500 },
        { participantId: 'beto', amount: 2700, splitValue: 0 },
        { participantId: 'carla', amount: 2100, splitValue: -600 },
      ],
    })
  })

  it('gives the leftover of the equal part to the lowest ids', () => {
    const adjustments = [
      { participantId: 'carla', value: 100 },
      { participantId: 'ana', value: 0 },
      { participantId: 'beto', value: 0 },
    ]
    expect(splitExpense(1001, { method: 'adjustment', adjustments })).toEqual({
      ok: true,
      value: [
        { participantId: 'carla', amount: 400, splitValue: 100 },
        { participantId: 'ana', amount: 301, splitValue: 0 },
        { participantId: 'beto', amount: 300, splitValue: 0 },
      ],
    })
  })

  it('allows adjustments larger than the total while nobody owes less than zero', () => {
    // 1.01 - 1.20 leaves -0.19 to split: -0.09 for Ana and -0.10 for Beto.
    const adjustments = [
      { participantId: 'ana', value: 60 },
      { participantId: 'beto', value: 60 },
    ]
    expect(splitExpense(101, { method: 'adjustment', adjustments })).toEqual({
      ok: true,
      value: [
        { participantId: 'ana', amount: 51, splitValue: 60 },
        { participantId: 'beto', amount: 50, splitValue: 60 },
      ],
    })
  })

  it('rejects adjustments that leave someone owing less than zero', () => {
    const adjustments = [
      { participantId: 'ana', value: -1200 },
      { participantId: 'beto', value: 0 },
    ]
    expect(splitExpense(1000, { method: 'adjustment', adjustments })).toEqual({
      ok: false,
      error: { code: 'negative-share', participantId: 'ana' },
    })
  })

  it('leaves out whoever the adjustment brings to exactly zero', () => {
    const adjustments = [
      { participantId: 'ana', value: -3000 },
      { participantId: 'beto', value: 0 },
      { participantId: 'carla', value: 0 },
    ]
    expect(splitExpense(6000, { method: 'adjustment', adjustments })).toEqual({
      ok: true,
      value: [
        { participantId: 'beto', amount: 3000, splitValue: 0 },
        { participantId: 'carla', amount: 3000, splitValue: 0 },
      ],
    })
  })

  it('only accepts whole minor units', () => {
    const adjustments = [
      { participantId: 'ana', value: 0 },
      { participantId: 'beto', value: 2.5 },
    ]
    expect(splitExpense(1000, { method: 'adjustment', adjustments })).toEqual({
      ok: false,
      error: { code: 'invalid-value', participantId: 'beto' },
    })
  })
})
