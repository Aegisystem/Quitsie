import { ok, type Result } from './result'
import { type ParticipantId, type ParticipantValue, type SplitError, splitExpense } from './split'

export interface PaidShare {
  participantId: ParticipantId
  amount: number
}

// What several payers put in follows the same rules as exact OwedShares: whole
// non-negative amounts that add up to the total. So it also fails with the same
// errors. Unlike an OwedShare, a PaidShare is never zero: whoever put in
// nothing did not pay.
export function multiplePayers(
  total: number,
  amounts: readonly ParticipantValue[],
): Result<PaidShare[], SplitError> {
  const shares = splitExpense(total, { method: 'exact', amounts })
  if (!shares.ok) return shares
  return ok(shares.value.filter(({ amount }) => amount > 0).map(({ participantId, amount }) => ({ participantId, amount })))
}

export function singlePayer(total: number, participantId: ParticipantId): Result<PaidShare[], SplitError> {
  return multiplePayers(total, [{ participantId, value: total }])
}
