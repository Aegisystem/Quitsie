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
