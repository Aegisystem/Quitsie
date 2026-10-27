import { useSyncExternalStore } from 'react'
import { computeBalances } from './balances'
import { createDemoData, type DemoData } from './data'
import type { Activity, Expense, Ledger, Member, Settlement } from './types'

// In-memory demo store for the v1 screens: no IndexedDB, no API. The real
// replica (web/src/local) and sync engine (web/src/sync) replace it later.

export interface DemoState extends DemoData {
  signedIn: boolean
  // Lets the demo show the offline states without turning the network off.
  simulateOffline: boolean
  forceInstallPrompt: boolean
  syncing: boolean
}

function initialState(): DemoState {
  return {
    ...createDemoData(),
    signedIn: readFlag('quitsie.demo.signedIn'),
    simulateOffline: false,
    forceInstallPrompt: false,
    syncing: false,
  }
}

function readFlag(key: string): boolean {
  try {
    return localStorage.getItem(key) === '1'
  } catch {
    return false
  }
}

function writeFlag(key: string, on: boolean) {
  try {
    if (on) localStorage.setItem(key, '1')
    else localStorage.removeItem(key)
  } catch {
    // Private mode or blocked storage: the flag just does not persist.
  }
}

let state = initialState()
const listeners = new Set<() => void>()

function set(next: Partial<DemoState>) {
  state = { ...state, ...next }
  for (const l of listeners) l()
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

export function useDemo(): DemoState {
  return useSyncExternalStore(subscribe, () => state)
}

let nextId = 1
const newId = (prefix: string) => `${prefix}-new-${nextId++}`

function myMember(s: DemoState, ledgerId: string): Member | undefined {
  return s.members.find((m) => m.ledgerId === ledgerId && m.userId === s.me.id && m.status === 'active')
}

function logActivity(s: DemoState, ledgerId: string, kind: Activity['kind'], subject?: string): Activity[] {
  const actor = myMember(s, ledgerId)
  if (!actor) return s.activities
  return [{ id: newId('a'), ledgerId, actorMemberId: actor.id, kind, subject, at: new Date().toISOString() }, ...s.activities]
}

export const demo = {
  signIn() {
    writeFlag('quitsie.demo.signedIn', true)
    set({ signedIn: true })
  },
  signOut() {
    writeFlag('quitsie.demo.signedIn', false)
    set({ signedIn: false })
  },
  setSimulateOffline(on: boolean) {
    set({ simulateOffline: on })
  },
  setForceInstallPrompt(on: boolean) {
    set({ forceInstallPrompt: on })
  },
  setPreferredCurrency(currency: string) {
    set({ me: { ...state.me, preferredCurrency: currency } })
  },
  reset() {
    set({ ...createDemoData(), simulateOffline: false, forceInstallPrompt: false, syncing: false })
  },

  addExpense(expense: Omit<Expense, 'id' | 'createdAt' | 'authorMemberId'>, fixesRejectedId?: string) {
    const author = myMember(state, expense.ledgerId)
    const full: Expense = { ...expense, id: newId('e'), createdAt: new Date().toISOString(), authorMemberId: author?.id ?? '' }
    set({
      expenses: [full, ...state.expenses],
      activities: logActivity(state, expense.ledgerId, 'expenseAdded', expense.description),
      rejected: fixesRejectedId ? state.rejected.filter((r) => r.id !== fixesRejectedId) : state.rejected,
    })
    return full
  },

  addSettlement(settlement: Omit<Settlement, 'id' | 'createdAt'>) {
    const to = state.members.find((m) => m.id === settlement.toMemberId)
    set({
      settlements: [{ ...settlement, id: newId('s'), createdAt: new Date().toISOString() }, ...state.settlements],
      activities: logActivity(state, settlement.ledgerId, 'settlementAdded', to?.name),
    })
  },

  restoreExpense(id: string) {
    const e = state.expenses.find((x) => x.id === id)
    set({ expenses: state.expenses.map((x) => (x.id === id ? { ...x, deletedAt: undefined } : x)) })
    if (e) set({ activities: logActivity(state, e.ledgerId, 'expenseEdited', e.description) })
  },

  setSimplifyDebts(ledgerId: string, on: boolean) {
    set({ ledgers: state.ledgers.map((l) => (l.id === ledgerId && l.kind === 'group' ? { ...l, simplifyDebts: on } : l)) })
  },

  regenerateInvite(ledgerId: string) {
    const token = Math.random().toString(36).slice(2, 8)
    set({ ledgers: state.ledgers.map((l) => (l.id === ledgerId ? { ...l, inviteToken: `${l.id.slice(2)}-${token}` } : l)) })
  },

  // Claim: the signed-in User links to a pending member and inherits its history.
  claim(memberId: string) {
    const m = state.members.find((x) => x.id === memberId)
    if (!m) return
    set({ members: state.members.map((x) => (x.id === memberId ? { ...x, userId: state.me.id, name: state.me.name, email: state.me.email } : x)) })
    set({ activities: logActivity(state, m.ledgerId, 'memberClaimed', m.name) })
  },

  joinAsNewMember(ledgerId: string) {
    const member: Member = { id: newId('m'), ledgerId, name: state.me.name, email: state.me.email, userId: state.me.id, status: 'active' }
    set({ members: [...state.members, member] })
    set({ activities: logActivity(state, ledgerId, 'memberAdded', state.me.name) })
  },

  discardRejected(id: string) {
    set({ rejected: state.rejected.filter((r) => r.id !== id) })
  },

  // Sends the queue: the demo server accepts everything that is still valid.
  sync() {
    const pending = state.expenses.some((e) => e.sync === 'unsynced') || state.settlements.some((s) => s.sync === 'unsynced')
    if (!pending || state.syncing) return
    set({ syncing: true })
    window.setTimeout(() => {
      set({
        syncing: false,
        expenses: state.expenses.map((e) => (e.sync === 'unsynced' ? { ...e, sync: 'synced' } : e)),
        settlements: state.settlements.map((s) => (s.sync === 'unsynced' ? { ...s, sync: 'synced' } : s)),
      })
    }, 1600)
  },
}

// Selectors

export function ledgerById(s: DemoState, id: string): Ledger | undefined {
  return s.ledgers.find((l) => l.id === id)
}

export function memberById(s: DemoState, id: string): Member | undefined {
  return s.members.find((m) => m.id === id)
}

export function myMemberId(s: DemoState, ledgerId: string): string | undefined {
  return myMember(s, ledgerId)?.id
}

export function ledgerMembers(s: DemoState, ledgerId: string): Member[] {
  return s.members.filter((m) => m.ledgerId === ledgerId)
}

export function ledgerName(s: DemoState, ledger: Ledger): string {
  return ledger.kind === 'group' ? ledger.name : (memberById(s, ledger.friendMemberId)?.name ?? '')
}

// The ledgers the User is an active member of: their Groups and Friendships.
export function myLedgers(s: DemoState): Ledger[] {
  return s.ledgers.filter((l) => myMember(s, l.id))
}

export function ledgerExpenses(s: DemoState, ledgerId: string): Expense[] {
  return s.expenses.filter((e) => e.ledgerId === ledgerId)
}

export function ledgerSettlements(s: DemoState, ledgerId: string): Settlement[] {
  return s.settlements.filter((x) => x.ledgerId === ledgerId)
}

export function ledgerBalances(s: DemoState, ledgerId: string) {
  return computeBalances(ledgerExpenses(s, ledgerId), ledgerSettlements(s, ledgerId))
}

// The User's Balance in one ledger, per currency (zeros dropped).
export function myLedgerBalance(s: DemoState, ledgerId: string): Map<string, number> {
  const id = myMemberId(s, ledgerId)
  const mine = id ? ledgerBalances(s, ledgerId).get(id) : undefined
  return new Map([...(mine ?? [])].filter(([, v]) => v !== 0))
}

// The User's total per currency across every Group and Friendship. Never converted.
export function myTotals(s: DemoState): Map<string, number> {
  const totals = new Map<string, number>()
  for (const l of myLedgers(s)) {
    for (const [currency, amount] of myLedgerBalance(s, l.id)) {
      totals.set(currency, (totals.get(currency) ?? 0) + amount)
    }
  }
  // Amounts in different currencies are not comparable, so the order is the
  // preferred currency first and then by code, never by size.
  const preferred = s.me.preferredCurrency
  return new Map(
    [...totals]
      .filter(([, v]) => v !== 0)
      .sort(([a], [b]) => (a === preferred ? -1 : b === preferred ? 1 : a.localeCompare(b))),
  )
}

export function unsyncedCount(s: DemoState): number {
  return s.expenses.filter((e) => e.sync === 'unsynced').length + s.settlements.filter((x) => x.sync === 'unsynced').length
}
