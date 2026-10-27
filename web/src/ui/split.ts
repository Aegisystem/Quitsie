import { type ParticipantValue, type Split, type SplitError, type SplitMethod, splitExpense } from '../domain'

// What the form needs to draw the Cuotas while the Usuario types. The amounts
// come from the domain's splitExpense (ADR 0003); this file only adds what the
// screen shows on top: who got the leftover minor units, how much of the total
// is covered, and why the split does not close yet.

export interface SplitParticipant {
  memberId: string
  included: boolean
  // exact: minor units; percentage: hundredths (10000 = 100 %); weights: integer
  // weight; adjustment: minor units added or taken; equal: unused.
  value: number
}

export interface PreviewShare {
  memberId: string
  amount: number
  // Minor units this participant received from the remainder of an uneven split.
  remainder: number
}

export type SplitIssue = 'noParticipants' | 'percentageNot100' | 'negativeShare' | 'noWeights' | 'invalidValue'

export interface SplitPreview {
  shares: PreviewShare[]
  assigned: number
  // What is left to reach the total; negative when the shares go over it.
  unassigned: number
  issue?: SplitIssue
}

export function previewOwedShares(
  total: number,
  method: SplitMethod,
  participants: readonly SplitParticipant[],
): SplitPreview {
  const included = participants.filter((p) => p.included)
  if (included.length === 0) return { shares: [], assigned: 0, unassigned: total, issue: 'noParticipants' }
  // A weight of zero means owing nothing, so it does not take part in the split.
  const splitting = method === 'weights' ? included.filter((p) => p.value > 0) : included
  if (splitting.length === 0) return { shares: [], assigned: 0, unassigned: total, issue: 'noWeights' }
  // While the amount is still empty there is nothing to split yet.
  if (total <= 0) return finish(total, zeroShares(included))

  const result = splitExpense(total, toSplit(method, splitting))
  if (!result.ok) return explain(total, method, included, result.error)

  const owed = new Map(result.value.map((s) => [s.participantId, s.amount]))
  const floors = floorParts(BigInt(total), method, splitting)
  return finish(
    total,
    included.map((p) => {
      const amount = owed.get(p.memberId) ?? 0
      const floor = floors.get(p.memberId)
      return { memberId: p.memberId, amount, remainder: floor === undefined ? 0 : Math.max(0, amount - floor) }
    }),
  )
}

function toSplit(method: SplitMethod, participants: readonly SplitParticipant[]): Split {
  const values: ParticipantValue[] = participants.map((p) => ({ participantId: p.memberId, value: p.value }))
  switch (method) {
    case 'equal':
      return { method, participantIds: participants.map((p) => p.memberId) }
    case 'exact':
      return { method, amounts: values }
    case 'percentage':
      return { method, percentages: values }
    case 'weights':
      return { method, weights: values }
    case 'adjustment':
      return { method, adjustments: values }
  }
}

// When the split does not close, the screen still shows what was typed so the
// Usuario can see what is missing or over.
function explain(
  total: number,
  method: SplitMethod,
  included: readonly SplitParticipant[],
  error: SplitError,
): SplitPreview {
  switch (error.code) {
    case 'sum-mismatch':
      if (method === 'exact') {
        return finish(total, included.map((p) => ({ memberId: p.memberId, amount: p.value, remainder: 0 })))
      }
      return {
        ...finish(
          total,
          included.map((p) => ({ memberId: p.memberId, amount: Math.round((total * p.value) / 10000), remainder: 0 })),
        ),
        issue: 'percentageNot100',
      }
    case 'negative-share':
      return { ...finish(total, zeroShares(included)), issue: 'negativeShare' }
    case 'no-participants':
      return { shares: [], assigned: 0, unassigned: total, issue: 'noParticipants' }
    case 'total-not-positive':
      return finish(total, zeroShares(included))
    case 'invalid-value':
    case 'duplicate-participant':
      return { ...finish(total, zeroShares(included)), issue: 'invalidValue' }
  }
}

// Each Participant's exact part rounded down, before the leftover minor units are
// handed out; the difference with their Cuota is what the screen marks with «+».
function floorParts(total: bigint, method: SplitMethod, participants: readonly SplitParticipant[]): Map<string, number> {
  const parts = new Map<string, number>()
  const n = BigInt(participants.length)
  switch (method) {
    case 'exact':
      return parts
    case 'equal':
      for (const p of participants) parts.set(p.memberId, Number(floorDiv(total, n)))
      return parts
    case 'percentage':
      for (const p of participants) parts.set(p.memberId, Number(floorDiv(total * BigInt(p.value), 10000n)))
      return parts
    case 'weights': {
      const sum = participants.reduce((acc, p) => acc + BigInt(p.value), 0n)
      for (const p of participants) parts.set(p.memberId, Number(floorDiv(total * BigInt(p.value), sum)))
      return parts
    }
    case 'adjustment': {
      const rest = total - participants.reduce((acc, p) => acc + BigInt(p.value), 0n)
      for (const p of participants) parts.set(p.memberId, Number(floorDiv(rest, n) + BigInt(p.value)))
      return parts
    }
  }
}

// BigInt division truncates towards zero; the parts need the floor.
function floorDiv(a: bigint, b: bigint): bigint {
  return a / b - (a % b < 0n ? 1n : 0n)
}

function zeroShares(participants: readonly SplitParticipant[]): PreviewShare[] {
  return participants.map((p) => ({ memberId: p.memberId, amount: 0, remainder: 0 }))
}

function finish(total: number, shares: PreviewShare[]): SplitPreview {
  const assigned = shares.reduce((s, x) => s + x.amount, 0)
  return { shares, assigned, unassigned: total - assigned }
}
