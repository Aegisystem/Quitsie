import type { Category, SplitMethod } from '../api/client'
import type { Ledger } from '../local'

// The shapes the screens work with: the contract's (openapi.yaml) and the
// replica's, under their glossary names (CONTEXT.md, ADR 0009).
export type { Activity, Category, Debt, Member, NewExpense, NewSettlement, SplitMethod, User } from '../api/client'
export type {
  ExpenseView,
  Ledger,
  LedgerSummary,
  QueuedRecord,
  RejectedRecord,
  RejectionReason,
  SettlementView,
  SyncState,
  UnsyncedRecord,
} from '../local'

export type Group = Extract<Ledger, { kind: 'group' }>
export type Friendship = Extract<Ledger, { kind: 'friendship' }>

export const splitMethods: readonly SplitMethod[] = ['equal', 'exact', 'percentage', 'weights', 'adjustment']

// The eleven Categories of the contract, in its order.
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
