import { describe, expect, it } from 'vitest'
import type { Member } from '../../api/client'
import type { LedgerSummary } from '../../local'
import { usersToBefriend } from './usersToBefriend'

const member = (ledgerId: string, name: string, userId?: string, status: Member['status'] = 'active'): Member => ({
  id: `${ledgerId}-${name}`,
  ledgerId,
  name,
  userId,
  status,
  version: 1,
})

const group = (id: string, members: Member[]): LedgerSummary => ({
  ledger: { kind: 'group', id, name: id, currency: 'EUR', simplifyDebts: true, createdAt: '', version: 1 },
  members,
  balances: [],
})

const friendship = (id: string, members: Member[]): LedgerSummary => ({
  ledger: { kind: 'friendship', id, createdAt: '', version: 1 },
  members,
  balances: [],
})

describe('users to befriend', () => {
  it('lists the Users in my Groups, with every Group we share', () => {
    const home = [
      group('Viaje', [member('Viaje', 'Ana', 'me'), member('Viaje', 'Beto', 'beto')]),
      group('Casa', [member('Casa', 'Ana', 'me'), member('Casa', 'Beto', 'beto'), member('Casa', 'Carla', 'carla')]),
    ]
    expect(usersToBefriend(home, 'me')).toEqual([
      { userId: 'beto', name: 'Beto', groups: ['Viaje', 'Casa'] },
      { userId: 'carla', name: 'Carla', groups: ['Casa'] },
    ])
  })

  it('leaves out pending and former Members, and whoever is already a Friend', () => {
    const home = [
      group('Viaje', [
        member('Viaje', 'Ana', 'me'),
        member('Viaje', 'Pendiente'),
        member('Viaje', 'Antiguo', 'antiguo', 'former'),
        member('Viaje', 'Beto', 'beto'),
      ]),
      friendship('f', [member('f', 'Ana', 'me'), member('f', 'Beto', 'beto')]),
    ]
    expect(usersToBefriend(home, 'me')).toEqual([])
  })

  it('ignores a Group I left: the server would not accept that Friendship', () => {
    const home = [group('Viaje', [member('Viaje', 'Ana', 'me', 'former'), member('Viaje', 'Beto', 'beto')])]
    expect(usersToBefriend(home, 'me')).toEqual([])
  })
})
