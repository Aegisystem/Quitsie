// Pure functions that turn a SplitMethod into OwedShares, distribute the
// remainder and compute provisional Balances (ADR 0003). The most tested code
// of the client.
export { type Balance, type ExpenseRecord, provisionalBalances, type Records, type SettlementRecord } from './balances'
export { type CurrencyCode, isCurrency, minorUnitDigits, offeredCurrencies } from './currencies'
export {
  formatAmount,
  formatAmountInput,
  type FormatOptions,
  type ParseError,
  parseAmount,
  parsePercentage,
} from './money'
export { multiplePayers, type PaidShare, singlePayer } from './paidShares'
export type { Result } from './result'
export {
  type OwedShare,
  type ParticipantId,
  type ParticipantValue,
  type Split,
  type SplitError,
  type SplitMethod,
  splitExpense,
} from './split'
