import { assertAmount } from './money'
import { fail, ok, type Result } from './result'

// Participants are Members of the Expense's ledger (ADR 0008), so they are
// identified by their Member id.
export type ParticipantId = string

export type Split = { method: 'equal'; participantIds: readonly ParticipantId[] }

export type SplitMethod = Split['method']

export interface OwedShare {
  participantId: ParticipantId
  amount: number
  // The Participant's input to the SplitMethod, stored to re-edit the Expense
  // (owed_shares.split_value): percentage in hundredths, weight, or adjustment
  // in minor units. Null for equal and exact.
  splitValue: number | null
}

export type SplitError =
  | { code: 'total-not-positive' }
  | { code: 'no-participants' }
  | { code: 'duplicate-participant'; participantId: ParticipantId }

function compareIds(a: ParticipantId, b: ParticipantId): number {
  return a < b ? -1 : a > b ? 1 : 0
}

// Largest remainder method: everyone gets the floor of their exact part of the
// total, and the minor units left over go one each to the largest remainders.
// Ties go to the lowest id, so the result never depends on the order on screen.
function apportion(
  total: bigint,
  weights: readonly { participantId: ParticipantId; weight: bigint }[],
): Map<ParticipantId, bigint> {
  const sum = weights.reduce((acc, { weight }) => acc + weight, 0n)
  const parts = weights.map(({ participantId, weight }) => {
    const exact = total * weight
    const floor = exact / sum
    return { participantId, floor, remainder: exact - floor * sum }
  })
  let leftover = total - parts.reduce((acc, { floor }) => acc + floor, 0n)
  const byRemainder = [...parts].sort(
    (a, b) => (a.remainder === b.remainder ? compareIds(a.participantId, b.participantId) : a.remainder > b.remainder ? -1 : 1),
  )
  const amounts = new Map(parts.map(({ participantId, floor }) => [participantId, floor]))
  for (const { participantId, floor } of byRemainder) {
    if (leftover === 0n) break
    amounts.set(participantId, floor + 1n)
    leftover--
  }
  return amounts
}

function findDuplicate(ids: readonly ParticipantId[]): ParticipantId | undefined {
  const seen = new Set<ParticipantId>()
  for (const id of ids) {
    if (seen.has(id)) return id
    seen.add(id)
  }
  return undefined
}

// Turns a SplitMethod into OwedShares that add up to exactly the total (ADR 0003).
// Whoever ends up owing nothing gets no OwedShare: a Participant owes something.
export function splitExpense(total: number, split: Split): Result<OwedShare[], SplitError> {
  assertAmount(total)
  if (total <= 0) return fail({ code: 'total-not-positive' })
  const ids = split.participantIds
  if (ids.length === 0) return fail({ code: 'no-participants' })
  const duplicate = findDuplicate(ids)
  if (duplicate !== undefined) return fail({ code: 'duplicate-participant', participantId: duplicate })

  const amounts = apportion(
    BigInt(total),
    ids.map((participantId) => ({ participantId, weight: 1n })),
  )
  return ok(
    ids
      .map((participantId) => ({ participantId, amount: Number(amounts.get(participantId)), splitValue: null }))
      .filter(({ amount }) => amount > 0),
  )
}
