import { describe, expect, it } from 'vitest'
import { payerStatus, payerSummary, type Say, splitStatus, splitSummary } from './splitSummary'

// Keys with their values, so each test reads which text the form would show.
const say: Say = {
  t: ((key: string, values?: Record<string, unknown>) => (values ? `${key} ${JSON.stringify(values)}` : key)) as Say['t'],
  nameOf: (id) => (id === 'ana' ? 'Tú' : id[0]?.toUpperCase() + id.slice(1)),
  money: (minor) => `$${(minor / 100).toFixed(2)}`,
}

describe('the line of who paid', () => {
  it('names the payer and the total', () => {
    expect(payerSummary({ many: false, paid: [{ memberId: 'ana', amount: 12000 }], me: 'ana' }, say)).toEqual({
      title: 'expenseForm.summary.youPaid',
      detail: '$120.00',
      problem: false,
    })
    expect(payerSummary({ many: false, paid: [{ memberId: 'beto', amount: 0 }], me: 'ana' }, say)).toEqual({
      title: 'expenseForm.summary.paid {"name":"Beto"}',
      problem: false,
    })
  })

  it('lists what each one paid when several did', () => {
    const paid = [
      { memberId: 'ana', amount: 8000 },
      { memberId: 'beto', amount: 4000 },
    ]
    expect(payerSummary({ many: true, paid, me: 'ana' }, say)).toEqual({
      title: 'expenseForm.summary.paidBy {"count":2}',
      detail: 'Tú $80.00 · Beto $40.00',
      problem: false,
    })
  })

  it('says what to fix instead of the amounts', () => {
    expect(payerSummary({ many: true, paid: [{ memberId: 'ana', amount: 8000 }], problem: 'left', me: 'ana' }, say)).toEqual({
      title: 'expenseForm.summary.severalPaid',
      detail: 'left',
      problem: true,
    })
  })
})

describe('the line of how it is divided', () => {
  const shares = [
    { memberId: 'ana', amount: 3000 },
    { memberId: 'beto', amount: 3000 },
    { memberId: 'carla', amount: 3000 },
    { memberId: 'dani', amount: 3000 },
  ]
  const line = (overrides: Partial<Parameters<typeof splitSummary>[0]>) =>
    splitSummary({ method: 'equal', takingPart: ['ana', 'beto', 'carla', 'dani'], everyone: 4, shares, me: 'ana', ...overrides }, say)

  it('says the method, among how many, and the Cuota each', () => {
    expect(line({})).toEqual({
      title: 'expenseForm.summary.amongAll {"method":"splitMethods.equal","count":4}',
      detail: 'expenseForm.summary.each {"amount":"$30.00"}',
      problem: false,
    })
  })

  it('counts who is left out', () => {
    const three = shares.slice(0, 3).map((share) => ({ ...share, amount: 4000 }))
    expect(line({ takingPart: ['ana', 'beto', 'carla'], shares: three }).title).toBe(
      'expenseForm.summary.amongSome {"method":"splitMethods.equal","count":3,"everyone":4}',
    )
  })

  it('lists the Cuotas when they differ', () => {
    const exact = [
      { memberId: 'ana', amount: 5000 },
      { memberId: 'beto', amount: 7000 },
    ]
    expect(line({ method: 'exact', takingPart: ['ana', 'beto'], shares: exact }).detail).toBe('Tú $50.00 · Beto $70.00')
  })

  it('gives everything to one person', () => {
    const one = [{ memberId: 'beto', amount: 12000 }]
    expect(line({ takingPart: ['beto'], shares: one })).toEqual({ title: 'expenseForm.summary.allFor {"name":"Beto"}', detail: '$120.00', problem: false })
    expect(line({ takingPart: ['ana'], shares: undefined })).toEqual({ title: 'expenseForm.summary.allYours', problem: false })
  })

  it('says what to fix instead of the Cuotas', () => {
    expect(line({ method: 'exact', shares: undefined, problem: 'missing' })).toMatchObject({ detail: 'missing', problem: true })
    expect(line({ takingPart: [], shares: undefined, problem: 'nobody' })).toEqual({
      title: 'expenseForm.summary.nobody',
      detail: 'nobody',
      problem: true,
    })
  })
})

describe('the status of the split', () => {
  const status = (overrides: Partial<Parameters<typeof splitStatus>[0]>) =>
    splitStatus({ total: 12000, method: 'equal', unassigned: 0, percentSum: 0, ...overrides }, say)

  it('asks for the amount before anything else', () => {
    expect(status({ total: 0, issue: 'noParticipants' })).toEqual({ tone: 'quiet', text: 'expenseForm.amountFirst' })
  })

  it('says how each method makes someone take part', () => {
    expect(status({ issue: 'noParticipants' }).text).toBe('expenseForm.noParticipants')
    expect(status({ method: 'exact', issue: 'noParticipants' }).text).toBe('expenseForm.noParticipantsExact')
    expect(status({ method: 'percentage', issue: 'noParticipants' }).text).toBe('expenseForm.noParticipantsPercentage')
    expect(status({ method: 'weights', issue: 'noParticipants' }).text).toBe('expenseForm.noWeights')
  })

  it('says what is missing or over', () => {
    expect(status({ method: 'exact', unassigned: 1500 })).toEqual({ tone: 'problem', text: 'expenseForm.missing {"amount":"$15.00"}' })
    expect(status({ method: 'exact', unassigned: -1000 })).toEqual({ tone: 'problem', text: 'expenseForm.over {"amount":"$10.00"}' })
    expect(status({ method: 'percentage', issue: 'percentageNot100', percentSum: 8000 }).text).toMatch(/^expenseForm.percentMissing/)
  })

  it('points at a field that cannot be read before the sums', () => {
    expect(status({ valueProblem: 'Revisa el valor de Beto.', unassigned: 100 }).text).toBe('Revisa el valor de Beto.')
  })

  it('closes with the total', () => {
    expect(status({})).toEqual({ tone: 'even', text: 'expenseForm.closes {"total":"$120.00"}' })
  })
})

describe('the status of the payers', () => {
  it('says what is left to cover, or that it adds up', () => {
    expect(payerStatus({ left: 4000 }, say)).toEqual({ tone: 'problem', text: 'expenseForm.payersLeft {"amount":"$40.00"}' })
    expect(payerStatus({ left: 0, paidProblem: 'Revisa lo que pagó Beto.' }, say).text).toBe('Revisa lo que pagó Beto.')
    expect(payerStatus({ left: 0 }, say)).toEqual({ tone: 'even', text: 'expenseForm.payersOk' })
  })
})
