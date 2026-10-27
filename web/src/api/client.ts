import createClient from 'openapi-fetch'
import type { components, paths } from './schema.gen'

type Schemas = components['schemas']

export type Health = Schemas['Health']

// Errors: every non-2xx response is a Problem; branch on `code`.
export type Problem = Schemas['Problem']
export type ProblemCode = Schemas['ProblemCode']
export type CurrencyPrecisionProblem = Schemas['CurrencyPrecisionProblem']

export type Currency = Schemas['Currency']
export type Amount = Schemas['Amount']
export type Version = Schemas['Version']
export type Category = Schemas['Category']
export type SplitMethod = Schemas['SplitMethod']

export type User = Schemas['User']
export type Session = Schemas['Session']

// What GET /sync returns, row by row, for the local replica.
export type Changes = Schemas['Changes']
export type Group = Schemas['Group']
export type Friendship = Schemas['Friendship']
export type Member = Schemas['Member']
export type MemberStatus = Schemas['MemberStatus']
export type Expense = Schemas['Expense']
export type PaidShare = Schemas['PaidShare']
export type OwedShare = Schemas['OwedShare']
export type Settlement = Schemas['Settlement']
export type Activity = Schemas['Activity']
export type Debt = Schemas['Debt']
export type LedgerDebts = Schemas['LedgerDebts']
export type DroppedLedger = Schemas['DroppedLedger']

// Unsynced records and what POST /records says about each one.
export type NewExpense = Schemas['NewExpense']
export type NewSettlement = Schemas['NewSettlement']
export type RecordBatch = Schemas['RecordBatch']
export type RecordResult = Schemas['RecordResult']

export type Invite = Schemas['Invite']
export type InviteLink = Schemas['InviteLink']
export type JoinedLedger = Schemas['JoinedLedger']

// Typed client for openapi.yaml. The API shares the PWA's origin, so there is
// no CORS and the session cookie travels on its own.
export const api = createClient<paths>({ baseUrl: '/api/v1' })
