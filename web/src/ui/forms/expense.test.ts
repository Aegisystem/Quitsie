import { describe, expect, it } from 'vitest'
import type { Member } from '../../api/client'
import { splitExpense } from '../../domain'
import { previewOwedShares } from '../split'
import {
  changedFormerShare,
  type ExpenseDraft,
  formerParticipant,
  formerShares,
  initialPaidText,
  hundredths,
  initialSplitValues,
  methodsFor,
  noFormerShares,
  switchMethod,
  toOwedShares,
} from './expense'

const member = (id: string, status: Member['status'] = 'active') => ({ id, status }) as Member

const draft = (overrides: Partial<ExpenseDraft>): ExpenseDraft => ({
  description: 'Cena',
  date: '2026-10-30',
  currency: 'USD',
  total: 9000,
  splitMethod: 'equal',
  paidShares: [{ memberId: 'ana', amount: 9000 }],
  owedShares: [
    { memberId: 'ana', amount: 3000 },
    { memberId: 'beto', amount: 3000 },
    { memberId: 'carla', amount: 3000 },
  ],
  ...overrides,
})

describe('toOwedShares', () => {
  it('keeps the Cuotas that round down to zero', () => {
    const split = splitExpense(1, { method: 'equal', participantIds: ['a', 'b', 'c'] })
    if (!split.ok) throw new Error('the split should close')
    expect(toOwedShares(split.value)).toEqual([
      { memberId: 'a', amount: 1 },
      { memberId: 'b', amount: 0 },
      { memberId: 'c', amount: 0 },
    ])
  })

  it('keeps the value in the method, and sends none for equal or exact', () => {
    const split = splitExpense(300, {
      method: 'weights',
      weights: [
        { participantId: 'a', value: 2 },
        { participantId: 'b', value: 1 },
      ],
    })
    if (!split.ok) throw new Error('the split should close')
    expect(toOwedShares(split.value)).toEqual([
      { memberId: 'a', amount: 200, splitValue: 2 },
      { memberId: 'b', amount: 100, splitValue: 1 },
    ])
  })
})

describe('initialSplitValues', () => {
  it('starts every method empty in a new Expense', () => {
    expect(initialSplitValues()).toEqual({ equal: {}, exact: {}, percentage: {}, weights: {}, adjustment: {} })
  })

  it('brings back what each Participant had in the method', () => {
    const values = initialSplitValues(
      draft({
        splitMethod: 'percentage',
        owedShares: [
          { memberId: 'a', amount: 2250, splitValue: 2500 },
          { memberId: 'b', amount: 6750, splitValue: 7500 },
        ],
      }),
    )
    expect(values.percentage).toEqual({ a: '25', b: '75' })
  })

  it('leaves out of the weights whoever was not in the split', () => {
    const values = initialSplitValues(
      draft({
        splitMethod: 'weights',
        owedShares: [
          { memberId: 'a', amount: 6000, splitValue: 2 },
          { memberId: 'b', amount: 3000, splitValue: 1 },
        ],
      }),
    )
    expect(values.weights).toEqual({ a: '2', b: '1' })
  })

  it('offers the Cuotas as exact amounts, whatever the saved method', () => {
    expect(initialSplitValues(draft({})).exact).toEqual({ ana: '30.00', beto: '30.00', carla: '30.00' })
  })
})

describe('switchMethod', () => {
  const activeIds = ['ana', 'beto', 'carla', 'dani']
  const blank = initialSplitValues()
  const everyone = { ana: true, beto: true, carla: true, dani: true }
  const shares = new Map([
    ['ana', 4000],
    ['beto', 4000],
    ['carla', 4000],
  ])
  const from = (overrides: Partial<Parameters<typeof switchMethod>[1]>) => ({
    values: blank,
    included: everyone,
    activeIds,
    takingPart: ['ana', 'beto', 'carla'],
    shares,
    currency: 'USD',
    ...overrides,
  })

  it('starts exact amounts from the Cuotas on screen, for whoever takes part', () => {
    const { values, included } = switchMethod('exact', from({}))
    expect(values.exact).toEqual({ ana: '40.00', beto: '40.00', carla: '40.00' })
    expect(included).toEqual({ ana: true, beto: true, carla: true, dani: false })
  })

  it('leaves exact amounts empty while the split does not close', () => {
    expect(switchMethod('exact', from({ shares: undefined })).values.exact).toEqual({})
  })

  it('starts percentages that add up to exactly 100', () => {
    expect(switchMethod('percentage', from({})).values.percentage).toEqual({ ana: '33.34', beto: '33.33', carla: '33.33' })
    const uneven = new Map([
      ['ana', 9000],
      ['beto', 3000],
    ])
    expect(switchMethod('percentage', from({ takingPart: ['ana', 'beto'], shares: uneven })).values.percentage).toEqual({ ana: '75', beto: '25' })
  })

  it('starts one part each in weights', () => {
    expect(switchMethod('weights', from({})).values.weights).toEqual({ ana: '1', beto: '1', carla: '1' })
  })

  it('keeps what was typed in the method', () => {
    const typed = { ...blank, exact: { ana: '10' } }
    expect(switchMethod('exact', from({ values: typed })).values.exact).toEqual({ ana: '10' })
  })

  it('ticks, for equal parts and adjustments, whoever took part', () => {
    const exact = { ...blank, exact: { beto: '70', dani: '50' } }
    const { values, included } = switchMethod('equal', from({ values: exact, takingPart: ['beto', 'dani'], shares: undefined }))
    expect(included).toEqual({ ana: false, beto: true, carla: false, dani: true })
    expect(values).toBe(exact)
  })

  it('changes nothing when nobody takes part', () => {
    const none = { ana: false, beto: false, carla: false, dani: false }
    expect(switchMethod('weights', from({ included: none, takingPart: [] }))).toEqual({ values: blank, included: none })
  })
})

describe('hundredths', () => {
  it('gives the hundredths left to the largest remainders, the first on a tie', () => {
    expect(hundredths([1, 1, 1])).toEqual([3334, 3333, 3333])
    expect(hundredths([1, 2])).toEqual([3333, 6667])
    expect(hundredths([0, 0])).toEqual([5000, 5000])
    expect(hundredths([10 ** 15, 1])).toEqual([10000, 0])
  })
})

describe('initialPaidText', () => {
  it('brings back every Aporte', () => {
    const paidShares = [
      { memberId: 'ana', amount: 6000 },
      { memberId: 'beto', amount: 3000 },
    ]
    expect(initialPaidText(draft({ paidShares }))).toEqual({ ana: '60.00', beto: '30.00' })
  })
})

describe('a former member in an Expense', () => {
  const members = [member('ana'), member('beto'), member('carla', 'former')]
  const former = formerShares(draft({}), members)

  it('has their Aportes and Cuotas kept, and nobody else’s', () => {
    expect([...former.paid.keys()]).toEqual([])
    expect([...former.owed.keys()]).toEqual(['carla'])
    const payer = formerShares(draft({ paidShares: [{ memberId: 'carla', amount: 9000 }] }), members)
    expect(payer.paid.get('carla')).toBe(9000)
  })

  it('counts with a Cuota of zero, which is still taking part', () => {
    const owedShares = [
      { memberId: 'ana', amount: 1 },
      { memberId: 'carla', amount: 0 },
    ]
    expect(formerShares(draft({ total: 1, owedShares }), members).owed.get('carla')?.amount).toBe(0)
  })

  it('limits the methods to the saved one and exact', () => {
    expect(methodsFor('percentage', former)).toEqual(['exact', 'percentage'])
    expect(methodsFor('percentage', noFormerShares)).toHaveLength(5)
  })

  it('leaves the methods alone if they only paid', () => {
    const owedShares = [{ memberId: 'ana', amount: 9000 }]
    const payer = formerShares(draft({ paidShares: [{ memberId: 'carla', amount: 9000 }], owedShares }), members)
    expect(methodsFor('equal', payer)).toHaveLength(5)
  })

  it('takes part with what was saved: the amount in exact, the value elsewhere', () => {
    const share = { memberId: 'carla', amount: 2250, splitValue: 2500 }
    expect(formerParticipant(share, 'exact')).toEqual({ memberId: 'carla', included: true, value: 2250 })
    expect(formerParticipant(share, 'percentage')).toEqual({ memberId: 'carla', included: true, value: 2500 })
  })

  const equalAmong = (total: number, ...ids: string[]) => {
    const participants = ids.map((memberId) => ({ memberId, included: true, value: 0 }))
    return previewOwedShares(total, 'equal', participants).owedShares ?? []
  }

  it('passes when the split gives them what they had', () => {
    expect(changedFormerShare(former, equalAmong(9000, 'ana', 'beto', 'carla'))).toBeUndefined()
  })

  it('is caught when a new total would change their Cuota', () => {
    expect(changedFormerShare(former, equalAmong(9300, 'ana', 'beto', 'carla'))).toEqual({ memberId: 'carla', from: 3000, to: 3100 })
  })

  it('is caught when they would drop out of the split', () => {
    expect(changedFormerShare(former, equalAmong(9000, 'ana', 'beto'))).toEqual({ memberId: 'carla', from: 3000, to: 0 })
  })

  it('can change the total with exact amounts, keeping their amount', () => {
    const participants = [
      formerParticipant({ memberId: 'carla', amount: 3000 }, 'exact'),
      { memberId: 'ana', included: true, value: 3150 },
      { memberId: 'beto', included: true, value: 3150 },
    ]
    const preview = previewOwedShares(9300, 'exact', participants)
    expect(changedFormerShare(former, preview.owedShares ?? [])).toBeUndefined()
    expect(preview.unassigned).toBe(0)
  })

  it('keeps a Cuota of zero in exact amounts', () => {
    const zero = formerShares(
      draft({
        total: 100,
        splitMethod: 'exact',
        owedShares: [
          { memberId: 'ana', amount: 100 },
          { memberId: 'carla', amount: 0 },
        ],
      }),
      members,
    )
    const participants = [formerParticipant({ memberId: 'carla', amount: 0 }, 'exact'), { memberId: 'ana', included: true, value: 100 }]
    const owed = previewOwedShares(100, 'exact', participants).owedShares ?? []
    expect(owed.find(({ participantId }) => participantId === 'carla')?.amount).toBe(0)
    expect(changedFormerShare(zero, owed)).toBeUndefined()
  })
})
