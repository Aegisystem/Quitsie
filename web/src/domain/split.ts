import { assertAmount } from './money'
import { fail, ok, type Result } from './result'

// Participants are Members of the Expense's ledger (ADR 0008), so they are
// identified by their Member id.
export type ParticipantId = string

// What the Usuario typed for one Participant: their amount in minor units for
// exact, their percentage in hundredths, their weight, or their adjustment.
export interface ParticipantValue {
  participantId: ParticipantId
  value: number
}

export type Split =
  | { method: 'equal'; participantIds: readonly ParticipantId[] }
  | { method: 'exact'; amounts: readonly ParticipantValue[] }
  | { method: 'percentage'; percentages: readonly ParticipantValue[] }
  | { method: 'weights'; weights: readonly ParticipantValue[] }

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
  // A value the SplitMethod does not accept, such as a negative exact amount.
  | { code: 'invalid-value'; participantId: ParticipantId }
  // How much the values add up to minus what they should: the total for exact,
  // 10000 hundredths for percentage.
  | { code: 'sum-mismatch'; difference: number }

function compareIds(a: ParticipantId, b: ParticipantId): number {
  return a < b ? -1 : a > b ? 1 : 0
}

// Largest remainder method: everyone gets the floor of their exact part of the
// total, in proportion to their value, and the minor units left over go one each
// to the largest remainders. Ties go to the lowest id, so the result never
// depends on the order on screen.
function apportion(
  total: bigint,
  weights: readonly ParticipantValue[],
): (ParticipantValue & { amount: bigint })[] {
  const sum = weights.reduce((acc, { value }) => acc + BigInt(value), 0n)
  const parts = weights.map((weight) => {
    const exact = total * BigInt(weight.value)
    const floor = exact / sum
    return { weight, floor, remainder: exact - floor * sum }
  })
  const leftover = total - parts.reduce((acc, { floor }) => acc + floor, 0n)
  const ranked = [...parts].sort((a, b) =>
    a.remainder === b.remainder
      ? compareIds(a.weight.participantId, b.weight.participantId)
      : a.remainder > b.remainder
        ? -1
        : 1,
  )
  const extra = new Set(ranked.slice(0, Number(leftover)))
  return parts.map((part) => ({ ...part.weight, amount: extra.has(part) ? part.floor + 1n : part.floor }))
}

function findDuplicate(ids: readonly ParticipantId[]): ParticipantId | undefined {
  const seen = new Set<ParticipantId>()
  for (const id of ids) {
    if (seen.has(id)) return id
    seen.add(id)
  }
  return undefined
}

function participantsOf(split: Split): readonly ParticipantId[] {
  switch (split.method) {
    case 'equal':
      return split.participantIds
    case 'exact':
      return split.amounts.map(({ participantId }) => participantId)
    case 'percentage':
      return split.percentages.map(({ participantId }) => participantId)
    case 'weights':
      return split.weights.map(({ participantId }) => participantId)
  }
}

type Owed = { participantId: ParticipantId; amount: bigint; splitValue: number | null }

function invalidValue(
  values: readonly ParticipantValue[],
  isValid: (value: number) => boolean,
): SplitError | undefined {
  const invalid = values.find(({ value }) => !Number.isSafeInteger(value) || !isValid(value))
  return invalid && { code: 'invalid-value', participantId: invalid.participantId }
}

function sumOf(values: readonly ParticipantValue[]): bigint {
  return values.reduce((acc, { value }) => acc + BigInt(value), 0n)
}

// 100 % in hundredths, the unit in which percentages are typed and stored.
const wholePercentage = 10000n

function proportional(total: bigint, values: readonly ParticipantValue[]): Owed[] {
  return apportion(total, values).map(({ participantId, value, amount }) => ({ participantId, amount, splitValue: value }))
}

function owe(total: bigint, split: Split): Result<Owed[], SplitError> {
  switch (split.method) {
    case 'equal': {
      const weights = split.participantIds.map((participantId) => ({ participantId, value: 1 }))
      return ok(apportion(total, weights).map(({ participantId, amount }) => ({ participantId, amount, splitValue: null })))
    }
    case 'exact': {
      const invalid = invalidValue(split.amounts, (value) => value >= 0)
      if (invalid) return fail(invalid)
      const difference = sumOf(split.amounts) - total
      if (difference !== 0n) return fail({ code: 'sum-mismatch', difference: Number(difference) })
      return ok(split.amounts.map(({ participantId, value }) => ({ participantId, amount: BigInt(value), splitValue: null })))
    }
    case 'percentage': {
      const invalid = invalidValue(split.percentages, (value) => value >= 0)
      if (invalid) return fail(invalid)
      const difference = sumOf(split.percentages) - wholePercentage
      if (difference !== 0n) return fail({ code: 'sum-mismatch', difference: Number(difference) })
      return ok(proportional(total, split.percentages))
    }
    case 'weights': {
      const invalid = invalidValue(split.weights, (value) => value > 0)
      if (invalid) return fail(invalid)
      return ok(proportional(total, split.weights))
    }
  }
}

// Turns a SplitMethod into OwedShares that add up to exactly the total (ADR 0003).
// Whoever ends up owing nothing gets no OwedShare: a Participant owes something.
export function splitExpense(total: number, split: Split): Result<OwedShare[], SplitError> {
  assertAmount(total)
  if (total <= 0) return fail({ code: 'total-not-positive' })
  const ids = participantsOf(split)
  if (ids.length === 0) return fail({ code: 'no-participants' })
  const duplicate = findDuplicate(ids)
  if (duplicate !== undefined) return fail({ code: 'duplicate-participant', participantId: duplicate })

  const owed = owe(BigInt(total), split)
  if (!owed.ok) return owed
  return ok(
    owed.value
      .filter(({ amount }) => amount > 0n)
      .map(({ participantId, amount, splitValue }) => ({ participantId, amount: Number(amount), splitValue })),
  )
}
