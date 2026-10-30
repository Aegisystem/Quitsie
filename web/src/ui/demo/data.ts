import { previewOwedShares } from '../split'
import type {
  Activity,
  ActivityKind,
  Category,
  Expense,
  Ledger,
  Member,
  RejectedRecord,
  Settlement,
  SplitMethod,
  SyncState,
  User,
} from './types'

// Demo data for the v1 screens. Nothing here is real; it lives only in memory.

export interface DemoData {
  me: User
  ledgers: Ledger[]
  members: Member[]
  expenses: Expense[]
  settlements: Settlement[]
  activities: Activity[]
  rejected: RejectedRecord[]
}

export function createDemoData(now = new Date()): DemoData {
  const day = (ago: number) => {
    const d = new Date(now)
    d.setDate(d.getDate() - ago)
    return d.toISOString().slice(0, 10)
  }
  const stamp = (minutesAgo: number) => new Date(now.getTime() - minutesAgo * 60_000).toISOString()

  const me: User = { id: 'u-sofia', name: 'Sofía', email: 'sofia@example.com', preferredCurrency: 'EUR' }

  const ledgers: Ledger[] = [
    { kind: 'group', id: 'g-lisboa', name: 'Viaje a Lisboa', currency: 'EUR', simplifyDebts: true, inviteToken: 'lisboa-7Qx2' },
    { kind: 'group', id: 'g-casa', name: 'Casa Rosales', currency: 'MXN', simplifyDebts: true, inviteToken: 'rosales-pT9' },
    { kind: 'group', id: 'g-valpo', name: 'Valparaíso 2026', currency: 'CLP', simplifyDebts: true, inviteToken: 'valpo-W4m' },
    // Sofía is not in this one yet: it is the Group behind the demo invite link.
    { kind: 'group', id: 'g-cumple', name: 'Cumpleaños de Diego', currency: 'EUR', simplifyDebts: true, inviteToken: 'cumple-K3d' },
    { kind: 'friendship', id: 'f-lucia', friendMemberId: 'm-fl-lucia' },
    { kind: 'friendship', id: 'f-martin', friendMemberId: 'm-fm-martin', inviteToken: 'martin-Zr8' },
    { kind: 'friendship', id: 'f-ana', friendMemberId: 'm-fa-ana' },
  ]

  const member = (id: string, ledgerId: string, name: string, extra: Partial<Member> = {}): Member => ({
    id,
    ledgerId,
    name,
    status: 'active',
    ...extra,
  })

  const members: Member[] = [
    member('m-l-sofia', 'g-lisboa', 'Sofía', { userId: me.id, email: me.email }),
    member('m-l-diego', 'g-lisboa', 'Diego', { userId: 'u-diego' }),
    member('m-l-camila', 'g-lisboa', 'Camila', { userId: 'u-camila' }),
    member('m-l-tomas', 'g-lisboa', 'Tomás', { email: 'tomas@example.com' }),

    member('m-c-sofia', 'g-casa', 'Sofía', { userId: me.id, email: me.email }),
    member('m-c-valentina', 'g-casa', 'Valentina', { userId: 'u-valentina' }),
    member('m-c-andres', 'g-casa', 'Andrés', { userId: 'u-andres' }),
    member('m-c-julia', 'g-casa', 'Julia', { userId: 'u-julia', status: 'former' }),

    member('m-v-sofia', 'g-valpo', 'Sofía', { userId: me.id, email: me.email }),
    member('m-v-mateo', 'g-valpo', 'Mateo', { userId: 'u-mateo' }),
    member('m-v-isabel', 'g-valpo', 'Isabel', { userId: 'u-isabel' }),

    member('m-k-diego', 'g-cumple', 'Diego', { userId: 'u-diego' }),
    member('m-k-camila', 'g-cumple', 'Camila', { userId: 'u-camila' }),
    member('m-k-sofi', 'g-cumple', 'Sofi', { email: me.email }),
    member('m-k-pablo', 'g-cumple', 'Pablo'),

    member('m-fl-sofia', 'f-lucia', 'Sofía', { userId: me.id }),
    member('m-fl-lucia', 'f-lucia', 'Lucía', { userId: 'u-lucia' }),
    member('m-fm-sofia', 'f-martin', 'Sofía', { userId: me.id }),
    member('m-fm-martin', 'f-martin', 'Martín'),
    member('m-fa-sofia', 'f-ana', 'Sofía', { userId: me.id }),
    member('m-fa-ana', 'f-ana', 'Ana', { userId: 'u-ana' }),
  ]

  let created = 60 * 24 * 12
  const expense = (e: {
    id: string
    ledgerId: string
    description: string
    category?: Category
    ago: number
    currency: string
    total: number
    method?: SplitMethod
    paid: [string, number][]
    split: [string, number][] | string[]
    author?: string
    sync?: SyncState
    deletedAt?: string
    createdAt?: string
  }): Expense => {
    const method = e.method ?? 'equal'
    const participants = e.split.map((s) =>
      typeof s === 'string' ? { memberId: s, included: true, value: 0 } : { memberId: s[0], included: true, value: s[1] },
    )
    const preview = previewOwedShares(e.total, method, participants)
    created -= 90
    return {
      id: e.id,
      ledgerId: e.ledgerId,
      description: e.description,
      date: day(e.ago),
      currency: e.currency,
      total: e.total,
      category: e.category,
      splitMethod: method,
      paidShares: e.paid.map(([memberId, amount]) => ({ memberId, amount })),
      owedShares: preview.shares.map((s, i) => ({
        memberId: s.memberId,
        amount: s.amount,
        splitValue: method === 'equal' || method === 'exact' ? undefined : participants[i]?.value,
      })),
      authorMemberId: e.author ?? e.paid[0]?.[0] ?? '',
      createdAt: e.createdAt ?? stamp(created),
      deletedAt: e.deletedAt,
      sync: e.sync ?? 'synced',
    }
  }

  const lisboa = ['m-l-sofia', 'm-l-diego', 'm-l-camila', 'm-l-tomas']
  const casa = ['m-c-sofia', 'm-c-valentina', 'm-c-andres']
  const valpo = ['m-v-sofia', 'm-v-mateo', 'm-v-isabel']

  const expenses: Expense[] = [
    // Viaje a Lisboa, EUR
    expense({ id: 'e-l-1', ledgerId: 'g-lisboa', description: 'Apartamento en Alfama', category: 'lodging', ago: 9, currency: 'EUR', total: 48000, paid: [['m-l-diego', 48000]], split: lisboa }),
    expense({ id: 'e-l-2', ledgerId: 'g-lisboa', description: 'Cena en el Barrio Alto', category: 'food', ago: 8, currency: 'EUR', total: 9640, paid: [['m-l-sofia', 9640]], split: lisboa }),
    expense({ id: 'e-l-3', ledgerId: 'g-lisboa', description: 'Supermercado', category: 'groceries', ago: 8, currency: 'EUR', total: 6317, paid: [['m-l-sofia', 4000], ['m-l-camila', 2317]], split: lisboa }),
    expense({ id: 'e-l-4', ledgerId: 'g-lisboa', description: 'Tranvía 28 y metro', category: 'transport', ago: 7, currency: 'EUR', total: 5200, method: 'exact', paid: [['m-l-camila', 5200]], split: [['m-l-sofia', 1300], ['m-l-diego', 1300], ['m-l-camila', 1300], ['m-l-tomas', 1300]] }),
    expense({ id: 'e-l-5', ledgerId: 'g-lisboa', description: 'Oceanário', category: 'entertainment', ago: 6, currency: 'EUR', total: 8800, method: 'percentage', paid: [['m-l-sofia', 8800]], split: [['m-l-sofia', 3000], ['m-l-diego', 3000], ['m-l-camila', 2000], ['m-l-tomas', 2000]] }),
    expense({ id: 'e-l-6', ledgerId: 'g-lisboa', description: 'Taxi al aeropuerto', category: 'transport', ago: 5, currency: 'EUR', total: 3200, paid: [['m-l-diego', 3200]], split: lisboa, deletedAt: stamp(60 * 30) }),
    expense({ id: 'e-l-7', ledgerId: 'g-lisboa', description: 'Pastéis de Belém', category: 'food', ago: 0, currency: 'EUR', total: 960, paid: [['m-l-sofia', 960]], split: lisboa, sync: 'unsynced', createdAt: stamp(22) }),

    // Casa Rosales, MXN
    expense({ id: 'e-c-1', ledgerId: 'g-casa', description: 'Renta de octubre', category: 'housing', ago: 26, currency: 'MXN', total: 1800000, method: 'weights', paid: [['m-c-valentina', 1800000]], split: [['m-c-sofia', 2], ['m-c-valentina', 1], ['m-c-andres', 1]] }),
    expense({ id: 'e-c-2', ledgerId: 'g-casa', description: 'Gas', category: 'utilities', ago: 24, currency: 'MXN', total: 48000, paid: [['m-c-julia', 48000]], split: [...casa, 'm-c-julia'] }),
    expense({ id: 'e-c-3', ledgerId: 'g-casa', description: 'Internet', category: 'utilities', ago: 12, currency: 'MXN', total: 65000, paid: [['m-c-andres', 65000]], split: casa }),
    expense({ id: 'e-c-4', ledgerId: 'g-casa', description: 'Súper de la semana', category: 'groceries', ago: 3, currency: 'MXN', total: 234050, method: 'adjustment', paid: [['m-c-sofia', 234050]], split: [['m-c-sofia', 0], ['m-c-valentina', 0], ['m-c-andres', 20000]] }),

    // Valparaíso 2026, CLP (no decimals)
    expense({ id: 'e-v-1', ledgerId: 'g-valpo', description: 'Hostal en Cerro Alegre', category: 'lodging', ago: 40, currency: 'CLP', total: 90000, paid: [['m-v-isabel', 90000]], split: valpo }),
    expense({ id: 'e-v-2', ledgerId: 'g-valpo', description: 'Empanadas y chorrillana', category: 'food', ago: 39, currency: 'CLP', total: 18000, paid: [['m-v-mateo', 18000]], split: valpo }),
    expense({ id: 'e-v-3', ledgerId: 'g-valpo', description: 'Ascensor y micro', category: 'transport', ago: 39, currency: 'CLP', total: 6000, paid: [['m-v-sofia', 6000]], split: valpo }),

    // Cumpleaños de Diego, EUR
    expense({ id: 'e-k-1', ledgerId: 'g-cumple', description: 'Tarta y velas', category: 'food', ago: 2, currency: 'EUR', total: 3600, paid: [['m-k-diego', 3600]], split: ['m-k-diego', 'm-k-camila', 'm-k-sofi', 'm-k-pablo'] }),
    expense({ id: 'e-k-2', ledgerId: 'g-cumple', description: 'Bebidas', category: 'groceries', ago: 2, currency: 'EUR', total: 5400, paid: [['m-k-camila', 5400]], split: ['m-k-diego', 'm-k-camila', 'm-k-sofi', 'm-k-pablo'] }),

    // Gastos sin grupo: each carries its own currency
    expense({ id: 'e-fl-1', ledgerId: 'f-lucia', description: 'Cine', category: 'entertainment', ago: 4, currency: 'EUR', total: 2500, paid: [['m-fl-sofia', 2500]], split: ['m-fl-sofia', 'm-fl-lucia'] }),
    expense({ id: 'e-fl-2', ledgerId: 'f-lucia', description: 'Libro a medias', category: 'shopping', ago: 15, currency: 'USD', total: 4000, paid: [['m-fl-lucia', 4000]], split: ['m-fl-sofia', 'm-fl-lucia'] }),
    expense({ id: 'e-fm-1', ledgerId: 'f-martin', description: 'Entradas del concierto', category: 'entertainment', ago: 11, currency: 'USD', total: 7000, paid: [['m-fm-sofia', 7000]], split: ['m-fm-sofia', 'm-fm-martin'] }),
    expense({ id: 'e-fa-1', ledgerId: 'f-ana', description: 'Café y medialunas', category: 'food', ago: 6, currency: 'EUR', total: 900, paid: [['m-fa-ana', 900]], split: ['m-fa-sofia', 'm-fa-ana'] }),
  ]

  const settlement = (
    id: string,
    ledgerId: string,
    fromMemberId: string,
    toMemberId: string,
    amount: number,
    currency: string,
    ago: number,
  ): Settlement => ({
    id,
    ledgerId,
    fromMemberId,
    toMemberId,
    amount,
    currency,
    date: day(ago),
    createdAt: stamp(ago * 60 * 24),
    sync: 'synced',
  })

  const settlements: Settlement[] = [
    settlement('s-l-1', 'g-lisboa', 'm-l-tomas', 'm-l-diego', 5000, 'EUR', 4),
    settlement('s-c-1', 'g-casa', 'm-c-sofia', 'm-c-julia', 12000, 'MXN', 20),
    settlement('s-c-2', 'g-casa', 'm-c-valentina', 'm-c-julia', 12000, 'MXN', 20),
    settlement('s-c-3', 'g-casa', 'm-c-andres', 'm-c-julia', 12000, 'MXN', 19),
    settlement('s-v-1', 'g-valpo', 'm-v-mateo', 'm-v-isabel', 20000, 'CLP', 35),
    settlement('s-v-2', 'g-valpo', 'm-v-sofia', 'm-v-isabel', 32000, 'CLP', 34),
    settlement('s-fa-1', 'f-ana', 'm-fa-sofia', 'm-fa-ana', 450, 'EUR', 5),
  ]

  const activity = (id: string, ledgerId: string, actorMemberId: string, kind: ActivityKind, minutesAgo: number, subject?: string): Activity => ({
    id,
    ledgerId,
    actorMemberId,
    kind,
    subject,
    at: stamp(minutesAgo),
  })

  const activities: Activity[] = [
    activity('a-l-1', 'g-lisboa', 'm-l-diego', 'groupCreated', 60 * 24 * 10),
    activity('a-l-2', 'g-lisboa', 'm-l-diego', 'memberAdded', 60 * 24 * 10 - 5, 'Tomás'),
    activity('a-l-3', 'g-lisboa', 'm-l-diego', 'expenseAdded', 60 * 24 * 9, 'Apartamento en Alfama'),
    activity('a-l-4', 'g-lisboa', 'm-l-sofia', 'expenseAdded', 60 * 24 * 8, 'Cena en el Barrio Alto'),
    activity('a-l-5', 'g-lisboa', 'm-l-camila', 'expenseAdded', 60 * 24 * 7, 'Tranvía 28 y metro'),
    activity('a-l-6', 'g-lisboa', 'm-l-sofia', 'expenseEdited', 60 * 24 * 6, 'Oceanário'),
    activity('a-l-7', 'g-lisboa', 'm-l-diego', 'settlementAdded', 60 * 24 * 4, 'Tomás'),
    activity('a-l-8', 'g-lisboa', 'm-l-diego', 'expenseDeleted', 60 * 30, 'Taxi al aeropuerto'),
    activity('a-c-1', 'g-casa', 'm-c-valentina', 'currencyChanged', 60 * 26, 'MXN'),
    activity('a-c-2', 'g-casa', 'm-c-julia', 'memberLeft', 60 * 24 * 18),
    activity('a-c-3', 'g-casa', 'm-c-sofia', 'expenseAdded', 60 * 24 * 3, 'Súper de la semana'),
    activity('a-v-1', 'g-valpo', 'm-v-sofia', 'settlementAdded', 60 * 24 * 34, 'Isabel'),
    activity('a-fl-1', 'f-lucia', 'm-fl-sofia', 'expenseAdded', 60 * 24 * 4, 'Cine'),
    activity('a-fm-1', 'f-martin', 'm-fm-sofia', 'friendAdded', 60 * 24 * 11, 'Martín'),
    activity('a-fa-1', 'f-ana', 'm-fa-sofia', 'settlementAdded', 60 * 24 * 5, 'Ana'),
  ]

  // Created offline before Valentina changed the Group's currency from USD to MXN.
  const rejected: RejectedRecord[] = [
    {
      id: 'r-1',
      reason: { code: 'currencyChanged', from: 'USD', to: 'MXN' },
      rejectedAt: stamp(25),
      record: {
        type: 'expense',
        expense: expense({ id: 'e-c-r1', ledgerId: 'g-casa', description: 'Lavandería', category: 'housing', ago: 1, currency: 'USD', total: 1200, paid: [['m-c-sofia', 1200]], split: casa, sync: 'unsynced', createdAt: stamp(60 * 27) }),
      },
    },
  ]

  return { me, ledgers, members, expenses, settlements, activities, rejected }
}
