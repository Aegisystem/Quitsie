import { describe, expect, it } from 'vitest'
import { problemPointer, readValue, valueError } from './values'

// The tests run in Node, whose region is en-US: "." separates the decimals.
const t = ((key: string, values?: Record<string, unknown>) => (values ? `${key} ${JSON.stringify(values)}` : key)) as Parameters<typeof valueError>[3]

describe('a person’s field', () => {
  it('is zero when empty, which leaves that person out', () => {
    for (const field of ['paid', 'exact', 'percentage', 'weights', 'adjustment'] as const) {
      expect(readValue('  ', field, 'USD')).toEqual({ ok: true, value: 0 })
    }
  })

  it('reads amounts in minor units, and percentages in hundredths', () => {
    expect(readValue('30.5', 'exact', 'USD')).toEqual({ ok: true, value: 3050 })
    expect(readValue('12.50', 'paid', 'USD')).toEqual({ ok: true, value: 1250 })
    expect(readValue('33.33 %', 'percentage', 'USD')).toEqual({ ok: true, value: 3333 })
  })

  it('never takes what it cannot read as zero', () => {
    expect(readValue('3o', 'exact', 'USD')).toEqual({ ok: false, error: 'invalid' })
    expect(readValue('treinta', 'paid', 'USD')).toEqual({ ok: false, error: 'invalid' })
    expect(readValue('10.555', 'adjustment', 'USD')).toEqual({ ok: false, error: 'too-many-decimals' })
    expect(readValue('5', 'exact', 'JPY')).toEqual({ ok: true, value: 5 })
    expect(readValue('5.5', 'exact', 'JPY')).toEqual({ ok: false, error: 'too-many-decimals' })
    expect(readValue('33.333', 'percentage', 'USD')).toEqual({ ok: false, error: 'too-many-decimals' })
  })

  it('takes a sign in an adjustment, but not a sign alone', () => {
    expect(readValue('+5', 'adjustment', 'USD')).toEqual({ ok: true, value: 500 })
    expect(readValue('-5', 'adjustment', 'USD')).toEqual({ ok: true, value: -500 })
    expect(readValue('+', 'adjustment', 'USD')).toEqual({ ok: false, error: 'invalid' })
    expect(readValue('-', 'adjustment', 'USD')).toEqual({ ok: false, error: 'invalid' })
  })

  it('does not let an Aporte below zero drop out of the payers', () => {
    expect(readValue('-10', 'paid', 'USD')).toEqual({ ok: false, error: 'negative' })
  })

  it('reads weights as whole numbers only', () => {
    expect(readValue('2', 'weights', 'USD')).toEqual({ ok: true, value: 2 })
    expect(readValue('0', 'weights', 'USD')).toEqual({ ok: true, value: 0 })
    expect(readValue('1.5', 'weights', 'USD')).toEqual({ ok: false, error: 'not-whole' })
    expect(readValue('1,5', 'weights', 'USD')).toEqual({ ok: false, error: 'not-whole' })
    expect(readValue('-1', 'weights', 'USD')).toEqual({ ok: false, error: 'invalid' })
    expect(readValue('2a', 'weights', 'USD')).toEqual({ ok: false, error: 'invalid' })
    expect(readValue('99999999999999999999', 'weights', 'USD')).toEqual({ ok: false, error: 'too-large' })
  })
})

describe('what a person’s field says when it cannot be read', () => {
  it('explains an amount as the total does', () => {
    expect(valueError('invalid', 'exact', 'USD', t)).toBe('expenseForm.amountInvalid {"example":"1234.50"}')
    expect(valueError('too-many-decimals', 'paid', 'JPY', t)).toBe('expenseForm.amountDecimals {"count":0,"currency":"JPY"}')
    expect(valueError('too-large', 'adjustment', 'USD', t)).toBe('expenseForm.amountTooLarge')
    expect(valueError('negative', 'paid', 'USD', t)).toBe('expenseForm.paidNegative')
  })

  it('has its own words for percentages and weights', () => {
    expect(valueError('invalid', 'percentage', 'USD', t)).toBe('expenseForm.percentInvalid {"example":"33.33"}')
    expect(valueError('too-many-decimals', 'percentage', 'USD', t)).toBe('expenseForm.percentDecimals')
    expect(valueError('invalid', 'weights', 'USD', t)).toBe('expenseForm.weightInvalid')
    expect(valueError('not-whole', 'weights', 'USD', t)).toBe('expenseForm.weightWhole {"example":"1.5"}')
    expect(valueError('too-large', 'weights', 'USD', t)).toBe('expenseForm.weightTooLarge')
  })
})

describe('the line away from a field that cannot be read', () => {
  it('only names whose field to look at', () => {
    expect(problemPointer('value', 'Ana', t)).toBe('expenseForm.valueProblem {"name":"Ana"}')
    expect(problemPointer('paid', 'Ana', t)).toBe('expenseForm.paidProblem {"name":"Ana"}')
  })

  it('speaks to the User about their own field', () => {
    expect(problemPointer('value', undefined, t)).toBe('expenseForm.valueProblemYours')
    expect(problemPointer('paid', undefined, t)).toBe('expenseForm.paidProblemYours')
  })
})
