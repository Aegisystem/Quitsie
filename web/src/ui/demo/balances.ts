import type { Expense, Settlement } from './types'

// Provisional Balances: plain sums over Expenses and Settlements, as the client
// shows them offline (ADR 0003). Deleted Expenses do not count.

// memberId → currency → amount (positive: owed to them; negative: they owe).
export type Balances = Map<string, Map<string, number>>

function add(balances: Balances, memberId: string, currency: string, amount: number) {
  let byCurrency = balances.get(memberId)
  if (!byCurrency) {
    byCurrency = new Map()
    balances.set(memberId, byCurrency)
  }
  byCurrency.set(currency, (byCurrency.get(currency) ?? 0) + amount)
}

export function computeBalances(expenses: readonly Expense[], settlements: readonly Settlement[]): Balances {
  const balances: Balances = new Map()
  for (const e of expenses) {
    if (e.deletedAt) continue
    for (const p of e.paidShares) add(balances, p.memberId, e.currency, p.amount)
    for (const o of e.owedShares) add(balances, o.memberId, e.currency, -o.amount)
  }
  for (const s of settlements) {
    add(balances, s.fromMemberId, s.currency, s.amount)
    add(balances, s.toMemberId, s.currency, -s.amount)
  }
  return balances
}

export interface Debt {
  fromMemberId: string
  toMemberId: string
  amount: number
}

// Fewest payments that clear every Balance in one currency: the largest debtor
// pays the largest creditor until one of them reaches zero. The server owns the
// real Debt simplification; this mirrors it for the demo.
export function simplifyDebts(balances: ReadonlyMap<string, number>): Debt[] {
  const debtors = [...balances].filter(([, v]) => v < 0).map(([id, v]) => ({ id, left: -v }))
  const creditors = [...balances].filter(([, v]) => v > 0).map(([id, v]) => ({ id, left: v }))
  const debts: Debt[] = []
  for (;;) {
    debtors.sort((a, b) => b.left - a.left)
    creditors.sort((a, b) => b.left - a.left)
    const d = debtors[0]
    const c = creditors[0]
    if (!d || !c || d.left === 0 || c.left === 0) break
    const amount = Math.min(d.left, c.left)
    debts.push({ fromMemberId: d.id, toMemberId: c.id, amount })
    d.left -= amount
    c.left -= amount
    if (d.left === 0) debtors.shift()
    if (c.left === 0) creditors.shift()
  }
  return debts
}

// Without simplification: who owes whom from each Expense, netted per pair.
export function pairwiseDebts(expenses: readonly Expense[], settlements: readonly Settlement[], currency: string): Debt[] {
  const net = new Map<string, number>()
  const key = (a: string, b: string) => `${a}→${b}`
  const owe = (from: string, to: string, amount: number) => {
    if (from === to || amount === 0) return
    net.set(key(from, to), (net.get(key(from, to)) ?? 0) + amount)
    net.set(key(to, from), (net.get(key(to, from)) ?? 0) - amount)
  }
  for (const e of expenses) {
    if (e.deletedAt || e.currency !== currency) continue
    for (const o of e.owedShares) {
      for (const p of e.paidShares) {
        owe(o.memberId, p.memberId, Math.round((o.amount * p.amount) / e.total))
      }
    }
  }
  for (const s of settlements) if (s.currency === currency) owe(s.toMemberId, s.fromMemberId, s.amount)
  const debts: Debt[] = []
  for (const [k, amount] of net) {
    if (amount <= 0) continue
    const [fromMemberId = '', toMemberId = ''] = k.split('→')
    debts.push({ fromMemberId, toMemberId, amount })
  }
  return debts.sort((a, b) => b.amount - a.amount)
}

export interface BalanceProofLine {
  id: string
  kind: 'expense' | 'settlement'
  description: string
  date: string
  currency: string
  // Effect on the member's Balance.
  amount: number
}

// What makes up one member's Balance: every Expense and Settlement that moves it.
export function balanceProof(
  memberId: string,
  expenses: readonly Expense[],
  settlements: readonly Settlement[],
  names: (memberId: string) => string,
): BalanceProofLine[] {
  const lines: BalanceProofLine[] = []
  for (const e of expenses) {
    if (e.deletedAt) continue
    const paid = e.paidShares.filter((p) => p.memberId === memberId).reduce((s, p) => s + p.amount, 0)
    const owed = e.owedShares.filter((o) => o.memberId === memberId).reduce((s, o) => s + o.amount, 0)
    if (paid === 0 && owed === 0) continue
    lines.push({ id: e.id, kind: 'expense', description: e.description, date: e.date, currency: e.currency, amount: paid - owed })
  }
  for (const s of settlements) {
    if (s.fromMemberId === memberId || s.toMemberId === memberId) {
      lines.push({
        id: s.id,
        kind: 'settlement',
        description: `${names(s.fromMemberId)} → ${names(s.toMemberId)}`,
        date: s.date,
        currency: s.currency,
        amount: s.fromMemberId === memberId ? s.amount : -s.amount,
      })
    }
  }
  return lines.sort((a, b) => b.date.localeCompare(a.date))
}
