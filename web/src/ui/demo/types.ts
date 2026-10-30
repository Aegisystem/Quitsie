// Demo-only shapes for the v1 screens. They follow the glossary's code names
// (CONTEXT.md, ADR 0009) so the screens can switch to the real replica later.

export type SplitMethod = 'equal' | 'exact' | 'percentage' | 'weights' | 'adjustment'

export const splitMethods: readonly SplitMethod[] = ['equal', 'exact', 'percentage', 'weights', 'adjustment']

// The eleven Categories of the contract (openapi.yaml), in its order.
export type Category =
  | 'food'
  | 'groceries'
  | 'transport'
  | 'lodging'
  | 'housing'
  | 'utilities'
  | 'entertainment'
  | 'shopping'
  | 'health'
  | 'gifts'
  | 'other'

export const categories: readonly Category[] = [
  'food',
  'groceries',
  'transport',
  'lodging',
  'housing',
  'utilities',
  'entertainment',
  'shopping',
  'health',
  'gifts',
  'other',
]

export interface User {
  id: string
  name: string
  email: string
  preferredCurrency: string
}

// A Member without userId is a pending member (or a pending friend, inside a
// Friendship's ledger, ADR 0008).
export interface Member {
  id: string
  ledgerId: string
  name: string
  email?: string
  userId?: string
  status: 'active' | 'former'
}

export interface Group {
  kind: 'group'
  id: string
  name: string
  currency: string
  simplifyDebts: boolean
  inviteToken: string
}

export interface Friendship {
  kind: 'friendship'
  id: string
  friendMemberId: string
  inviteToken?: string
}

export type Ledger = Group | Friendship

export type SyncState = 'synced' | 'unsynced'

export interface PaidShare {
  memberId: string
  amount: number
}

export interface OwedShare {
  memberId: string
  amount: number
  // The participant's value in the split method, kept to re-edit the Expense.
  splitValue?: number
}

export interface Expense {
  id: string
  ledgerId: string
  description: string
  date: string // YYYY-MM-DD
  currency: string
  total: number
  category?: Category
  splitMethod: SplitMethod
  paidShares: PaidShare[]
  owedShares: OwedShare[]
  authorMemberId: string
  createdAt: string
  deletedAt?: string
  sync: SyncState
}

export interface Settlement {
  id: string
  ledgerId: string
  fromMemberId: string
  toMemberId: string
  amount: number
  currency: string
  date: string
  createdAt: string
  sync: SyncState
}

export type ActivityKind =
  | 'groupCreated'
  | 'expenseAdded'
  | 'expenseEdited'
  | 'expenseDeleted'
  | 'settlementAdded'
  | 'memberAdded'
  | 'memberClaimed'
  | 'memberLeft'
  | 'currencyChanged'
  | 'simplifyEnabled'
  | 'friendAdded'

export interface Activity {
  id: string
  ledgerId: string
  actorMemberId: string
  kind: ActivityKind
  subject?: string
  at: string
}

export type RejectionReason = { code: 'currencyChanged'; from: string; to: string } | { code: 'notMember' }

export type UnsyncedRecord = { type: 'expense'; expense: Expense } | { type: 'settlement'; settlement: Settlement }

// An UnsyncedRecord the server refused. It waits for the User to fix or discard it.
export interface RejectedRecord {
  id: string
  record: UnsyncedRecord
  reason: RejectionReason
  rejectedAt: string
}
