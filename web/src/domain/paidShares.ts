import { ok, type Result } from './result'
import { type ParticipantId, type ParticipantValue, type SplitError, splitExpense } from './split'

export interface PaidShare {
  participantId: ParticipantId
  amount: number
}

// What several payers put in follows the same rules as exact OwedShares: whole
// non-negative amounts that add up to the total, and no PaidShare for whoever
// put in nothing. So it also fails with the same errors.
export function multiplePayers(
  total: number,
  amounts: readonly ParticipantValue[],
): Result<PaidShare[], SplitError> {
  const shares = splitExpense(total, { method: 'exact', amounts })
  if (!shares.ok) return shares
  return ok(shares.value.map(({ participantId, amount }) => ({ participantId, amount })))
}

export function singlePayer(total: number, participantId: ParticipantId): Result<PaidShare[], SplitError> {
  return multiplePayers(total, [{ participantId, value: total }])
}
