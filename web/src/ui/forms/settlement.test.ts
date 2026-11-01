import { describe, expect, it } from 'vitest'
import type { Member } from '../../api/client'
import { initialPeople, type SettlementFix, settlementRejectionText } from './settlement'

const member = (id: string, status: Member['status'] = 'active') => ({ id, status }) as Member
const me = member('yo')
const active = [me, member('ana'), member('beto')]

const t = ((key: string, values?: Record<string, string>) => (values ? `${key} ${JSON.stringify(values)}` : key)) as Parameters<
  typeof settlementRejectionText
>[2]

const fix = (overrides: Partial<SettlementFix> & { from?: string; to?: string } = {}): SettlementFix => ({
  id: 'rejected',
  reason: overrides.reason ?? 'invalid_participants',
  draft: { fromMemberId: overrides.from ?? 'yo', toMemberId: overrides.to ?? 'carla', amount: 4000, currency: 'USD', date: '2026-10-30' },
})

describe('who a Settlement starts with', () => {
  it('takes a suggested Debt as it comes', () => {
    expect(initialPeople(active, me, { from: 'beto', to: 'ana' })).toEqual({ from: 'beto', to: 'ana' })
  })

  it('starts with the User paying someone else when nothing is suggested', () => {
    expect(initialPeople(active, me, {})).toEqual({ from: 'yo', to: 'ana' })
  })

  it('leaves out of a suggestion whoever is no longer active', () => {
    expect(initialPeople(active, me, { from: 'carla', to: 'ana' })).toEqual({ from: 'yo', to: 'ana' })
    expect(initialPeople(active, me, { from: 'ana', to: 'carla' })).toEqual({ from: 'ana', to: 'yo' })
  })

  it('keeps who a rejected record had, so the form can say what is wrong', () => {
    expect(initialPeople(active, me, { from: 'beto', to: 'ana', fix: fix({ from: 'yo', to: 'carla' }) })).toEqual({ from: 'yo', to: 'carla' })
  })
})

describe('why a Settlement was rejected', () => {
  it('names both currencies when the Group changed its own', () => {
    expect(settlementRejectionText(fix({ reason: 'currency_mismatch' }), 'EUR', t)).toBe('settlement.rejectedCurrency {"from":"USD","to":"EUR"}')
  })

  it('points at the people when one of them left', () => {
    expect(settlementRejectionText(fix({ reason: 'invalid_participants' }), 'USD', t)).toBe('settlement.rejectedParticipants')
  })

  it('has a text for every other reason', () => {
    expect(settlementRejectionText(fix({ reason: 'id_conflict' }), 'USD', t)).toBe('settlement.rejectedOther')
    expect(settlementRejectionText(fix({ reason: 'currency_mismatch' }), undefined, t)).toBe('settlement.rejectedOther')
  })
})
