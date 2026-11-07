import { describe, expect, it } from 'vitest'
import { normalizeDescription, suggestCategory, type CategorizedExpense } from './category'

describe('normalizeDescription', () => {
  it('drops case, accents, punctuation and extra spaces', () => {
    expect(normalizeDescription('  Almuerzo   en El Corral!')).toBe('almuerzo en el corral')
    expect(normalizeDescription('Cumpleaños de Ána')).toBe('cumpleanos de ana')
    expect(normalizeDescription('Peaje—Bogotá/Girardot')).toBe('peaje bogota girardot')
  })
})

describe('suggestCategory', () => {
  it('takes the Category the Usuario gave the same description before', () => {
    const history: CategorizedExpense[] = [{ description: 'Mercado', category: 'housing' }]
    // The keyword list says groceries; the Usuario's own choice wins.
    expect(suggestCategory('mercado', history)).toBe('housing')
    expect(suggestCategory('  MERCADO ', history)).toBe('housing')
  })

  it('takes it from an almost equal description', () => {
    const history: CategorizedExpense[] = [{ description: 'Club de lectura', category: 'entertainment' }]
    expect(suggestCategory('Club de lectura noviembre', history)).toBe('entertainment')
  })

  it('takes the Category used most often, and the most recent on a tie', () => {
    const history: CategorizedExpense[] = [
      { description: 'Paseo', category: 'transport' },
      { description: 'Paseo', category: 'lodging' },
      { description: 'Paseo', category: 'lodging' },
    ]
    expect(suggestCategory('Paseo', history)).toBe('lodging')
    expect(
      suggestCategory('Paseo', [
        { description: 'Paseo', category: 'transport' },
        { description: 'Paseo', category: 'lodging' },
      ]),
    ).toBe('transport')
  })

  it('falls back to keywords, in the singular too', () => {
    expect(suggestCategory('Uber al aeropuerto', [])).toBe('transport')
    expect(suggestCategory('Almuerzo con Ana', [])).toBe('food')
    expect(suggestCategory('D1', [])).toBe('groceries')
    expect(suggestCategory('Recibo de la luz', [])).toBe('utilities')
    expect(suggestCategory('Tiquetes a Cartagena', [])).toBe('transport')
    expect(suggestCategory('Medicinas', [])).toBe('health')
    expect(suggestCategory('Regalo de cumpleaños', [])).toBe('gifts')
  })

  it('prefers a two-word name over its first word', () => {
    expect(suggestCategory('Pedido en Mercado Libre', [])).toBe('shopping')
    expect(suggestCategory('Didi Food', [])).toBe('food')
  })

  it('suggests nothing when nothing matches, or before there is anything to read', () => {
    expect(suggestCategory('Cosas varias', [])).toBeUndefined()
    expect(suggestCategory('a', [])).toBeUndefined()
    expect(suggestCategory('', [{ description: '', category: 'food' }])).toBeUndefined()
  })

  it('does not match on words that say nothing about the Expense', () => {
    const history: CategorizedExpense[] = [{ description: 'Cena con Ana', category: 'gifts' }]
    expect(suggestCategory('Taxi con Beto', history)).toBe('transport')
  })
})
