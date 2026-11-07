import { describe, expect, it } from 'vitest'
import { parseRoute } from '../ui/router'
import { ledgerPath, lineOf, noticeFor, type PushItem, type PushPayload, readNoticeData, readPayload } from './notification'

// Intl separates the symbol with a no-break space.
const plain = (text: string) => text.replace(/\u00a0/g, ' ')
const line = (item: PushItem, locale = 'es-MX') => plain(lineOf(item, locale))

const dinner: PushItem = { kind: 'expense', actor: 'Beto', description: 'Cena', amount: 9001, currency: 'USD', share: 4501 }

function trip(...items: PushItem[]): PushPayload {
  return { ledgerId: 'trip', title: 'Viaje a Cartagena', items }
}

describe('lineOf', () => {
  it('says who added an Expense, its total and the recipient’s part', () => {
    expect(line(dinner)).toBe('Beto añadió «Cena» por USD 90.01. Tu parte: USD 45.01.')
  })

  it('formats in the device’s region', () => {
    expect(line({ ...dinner, currency: 'COP', amount: 9000000, share: 4500000 }, 'es-CO')).toBe('Beto añadió «Cena» por $ 90.000. Tu parte: $ 45.000.')
  })

  it('tells each side of a Settlement who wrote it', () => {
    const paid = { kind: 'settlement', amount: 2000, currency: 'USD' } as const
    expect(line({ ...paid, actor: 'Ana', you: 'received', counterpart: 'Ana', byCounterpart: true })).toBe('Ana te pagó USD 20.')
    expect(line({ ...paid, actor: 'Carla', you: 'received', counterpart: 'Ana' })).toBe('Carla registró que Ana te pagó USD 20.')
    expect(line({ ...paid, actor: 'Beto', you: 'paid', counterpart: 'Beto', byCounterpart: true })).toBe('Beto registró que le pagaste USD 20.')
    expect(line({ ...paid, actor: 'Carla', you: 'paid', counterpart: 'Beto' })).toBe('Carla registró que le pagaste USD 20 a Beto.')
  })

  it('names nobody it does not know', () => {
    expect(line({ ...dinner, actor: '' })).toMatch(/^Alguien añadió/)
  })

  it('writes an amount of a currency it does not know', () => {
    expect(line({ ...dinner, currency: 'XQZ' })).toContain('9001 XQZ')
  })
})

describe('noticeFor', () => {
  it('is one notification per ledger, titled with its name', () => {
    const n = noticeFor(trip(dinner), undefined, 'es-MX')
    expect(n.title).toBe('Viaje a Cartagena')
    expect(n.tag).toBe('ledger:trip')
    expect(plain(n.body)).toBe('Beto añadió «Cena» por USD 90.01. Tu parte: USD 45.01.')
  })

  it('adds the next records to the notification still on screen', () => {
    const first = noticeFor(trip(dinner), undefined, 'es-MX')
    const taxi = { ...dinner, description: 'Taxi' }
    const next = noticeFor(trip(taxi), first.data, 'es-MX')
    expect(next.body.split('\n')).toEqual([lineOf(dinner, 'es-MX'), lineOf(taxi, 'es-MX')])
  })

  it('lists the newest records and counts the rest', () => {
    const items = ['A', 'B', 'C', 'D', 'E', 'F'].map((d) => ({ ...dinner, description: d }))
    const n = noticeFor({ ...trip(...items), more: 2 }, undefined, 'es-MX')
    const lines = n.body.split('\n')
    expect(lines).toHaveLength(5)
    expect(lines[0]).toContain('«C»')
    expect(lines[4]).toBe('Y 4 registros más.')
    expect(noticeFor({ ...trip(dinner), more: 1 }, undefined, 'es-MX').body).toMatch(/\nY 1 registro más\.$/)
  })
})

describe('reading what arrives', () => {
  it('keeps the records it understands', () => {
    const p = readPayload({ ledgerId: 'trip', title: 'Viaje', items: [dinner, { kind: 'poll' }, null], extra: true })
    expect(p).toEqual({ ledgerId: 'trip', title: 'Viaje', items: [dinner], more: 0 })
    expect(readPayload('nope')).toBeUndefined()
    expect(readPayload({ title: 'Viaje', items: [] })).toBeUndefined()
  })

  it('reads back the data of its own notifications only', () => {
    expect(readNoticeData(noticeFor(trip(dinner), undefined, 'es-MX').data)).toEqual({ ledgerId: 'trip', lines: [lineOf(dinner, 'es-MX')], more: 0 })
    expect(readNoticeData({ url: 'x' })).toBeUndefined()
  })
})

it('opens the ledger’s screen', () => {
  expect(parseRoute(new URL(ledgerPath('a/b'), 'https://quitsie.test').hash)).toEqual({ name: 'ledger', ledgerId: 'a/b', tab: undefined })
})
