import { createInstance } from 'i18next'
import { describe, expect, it } from 'vitest'
import { type Balance, maxBalance } from '../../domain'
import { es } from '../../i18n/es'
import { balanceError } from './amountError'
import { balanceRecords } from './expense'

const i18n = createInstance()
await i18n.init({ lng: 'es', resources: { es: { translation: es } }, interpolation: { escapeValue: false } })

const people = { me: 'ana', nameOf: (id: string) => (id === 'ana' ? 'Tú' : 'Beto') }
const max = 10 ** 15

// Ana paid `amount` for Beto.
const lent = (amount: number) =>
  balanceRecords({ currency: 'USD', paidShares: [{ memberId: 'ana', amount }], owedShares: [{ memberId: 'beto', amount }] })

// Ana is owed and Beto owes `amount`.
const at = (amount: number): Balance[] => [
  { participantId: 'ana', currency: 'USD', amount },
  { participantId: 'beto', currency: 'USD', amount: -amount },
]

describe('balanceError', () => {
  it('lets an Expense take the Balances exactly to the limit', () => {
    expect(balanceError(at(maxBalance - max), { added: lent(max) }, people, i18n.t)).toBeUndefined()
  })

  it('tells the User when their own Balance would go past it', () => {
    expect(balanceError(at(maxBalance - max + 1), { added: lent(max) }, people, i18n.t)).toBe(
      'Con esto, tu saldo pasaría de lo que Quitsie puede apuntar.',
    )
  })

  it('names someone else whose Balance would go past it', () => {
    const betoOwes: Balance[] = [{ participantId: 'beto', currency: 'USD', amount: -maxBalance }]
    expect(balanceError(betoOwes, { added: lent(1) }, people, i18n.t)).toBe(
      'Con esto, el saldo de Beto pasaría de lo que Quitsie puede apuntar.',
    )
  })

  it('checks a Settlement', () => {
    const settlement = { id: '', from: 'ana', to: 'beto', amount: 2, currency: 'USD' }
    expect(balanceError(at(maxBalance - 1), { added: { expenses: [], settlements: [settlement] } }, people, i18n.t)).toBeDefined()
  })

  it('counts the edited Expense once, taking out what it replaces', () => {
    expect(balanceError(at(maxBalance), { added: lent(max), removed: lent(max) }, people, i18n.t)).toBeUndefined()
    expect(balanceError(at(maxBalance), { added: lent(max + 1), removed: lent(max) }, people, i18n.t)).toBeDefined()
  })
})
