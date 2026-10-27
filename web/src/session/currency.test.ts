import { describe, expect, it } from 'vitest'
import { isCurrency } from '../domain'
import { currencyOptions, suggestCurrency } from './currency'

// ISO 4217 funds: units of account that nobody pays with.
const funds = ['BOV', 'CHE', 'CHW', 'CLF', 'COU', 'MXV', 'USN', 'UYI', 'UYW']

describe('suggestCurrency', () => {
  it('takes the currency of the language’s region', () => {
    expect(suggestCurrency('es-MX')).toBe('MXN')
    expect(suggestCurrency('es-CO')).toBe('COP')
    expect(suggestCurrency('es-CL')).toBe('CLP')
    expect(suggestCurrency('es-AR')).toBe('ARS')
    expect(suggestCurrency('en-US')).toBe('USD')
    expect(suggestCurrency('pt-BR')).toBe('BRL')
    expect(suggestCurrency('fr-FR')).toBe('EUR')
  })

  it('never proposes a fund, even where one exists', () => {
    for (const language of ['es-CL', 'es-CO', 'es-UY', 'es-BO', 'es-MX', 'de-CH', 'en-US']) {
      expect(funds).not.toContain(suggestCurrency(language))
    }
  })

  it('proposes the dollar where it is what prices are given in', () => {
    expect(suggestCurrency('es-EC')).toBe('USD')
    expect(suggestCurrency('es-SV')).toBe('USD')
    expect(suggestCurrency('es-PA')).toBe('USD')
  })

  it('takes the likely region of a language without one', () => {
    expect(suggestCurrency('es')).toBe('EUR')
    expect(suggestCurrency('ja')).toBe('JPY')
  })

  it('proposes nothing for a region without one currency or a tag it cannot read', () => {
    expect(suggestCurrency('es-419')).toBeUndefined()
    expect(suggestCurrency('')).toBeUndefined()
    expect(suggestCurrency('not a language')).toBeUndefined()
  })
})

describe('currencyOptions', () => {
  it('offers current ISO 4217 currencies only, sorted by code', () => {
    expect(currencyOptions.filter((c) => !isCurrency(c))).toEqual([])
    expect([...currencyOptions].sort()).toEqual(currencyOptions)
    expect(new Set(currencyOptions).size).toBe(currencyOptions.length)
  })

  it('never offers a fund', () => {
    expect(currencyOptions.filter((c) => funds.includes(c))).toEqual([])
  })

  it('offers the currencies of the demo data and of Latin America', () => {
    for (const c of ['EUR', 'USD', 'MXN', 'CLP', 'COP', 'ARS', 'PEN', 'UYU', 'BOB', 'BRL', 'GBP']) {
      expect(currencyOptions).toContain(c)
    }
  })
})
