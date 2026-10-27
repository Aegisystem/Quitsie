import type { SplitMethod } from './types'

// Stand-in for web/src/domain while that session writes the real split logic:
// same idea (total and Participants → OwedShares), so the form can switch to the
// domain function later without changing shape.

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

export type SplitIssue = 'noParticipants' | 'percentageNot100' | 'negativeShare' | 'noWeights'

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

  switch (method) {
    case 'equal':
      return finish(total, equalShares(total, included))

    case 'exact':
      return finish(
        total,
        included.map((p) => ({ memberId: p.memberId, amount: p.value, remainder: 0 })),
      )

    case 'percentage': {
      const sum = included.reduce((s, p) => s + p.value, 0)
      if (sum !== 10000) {
        const shares = included.map((p) => ({
          memberId: p.memberId,
          amount: Math.round((total * p.value) / 10000),
          remainder: 0,
        }))
        return { ...finish(total, shares), issue: 'percentageNot100' }
      }
      return finish(total, proportionalShares(total, included))
    }

    case 'weights': {
      if (included.every((p) => p.value <= 0)) {
        return { shares: [], assigned: 0, unassigned: total, issue: 'noWeights' }
      }
      return finish(total, proportionalShares(total, included))
    }

    case 'adjustment': {
      const adjustments = included.reduce((s, p) => s + p.value, 0)
      const base = equalShares(total - adjustments, included)
      const shares = base.map((s, i) => ({ ...s, amount: s.amount + (included[i]?.value ?? 0) }))
      const preview = finish(total, shares)
      return shares.some((s) => s.amount < 0) || total - adjustments < 0
        ? { ...preview, issue: 'negativeShare' }
        : preview
    }
  }
}

function finish(total: number, shares: PreviewShare[]): SplitPreview {
  const assigned = shares.reduce((s, x) => s + x.amount, 0)
  return { shares, assigned, unassigned: total - assigned }
}

// Even split; the leftover minor units go one each to the first participants in
// list order, so the result is deterministic.
function equalShares(total: number, included: readonly SplitParticipant[]): PreviewShare[] {
  const n = included.length
  const base = Math.floor(total / n)
  const left = total - base * n
  return included.map((p, i) => ({ memberId: p.memberId, amount: base + (i < left ? 1 : 0), remainder: i < left ? 1 : 0 }))
}

// Largest remainder: floor every share, then hand the leftover units to the
// largest fractional parts, ties broken by list order.
function proportionalShares(total: number, included: readonly SplitParticipant[]): PreviewShare[] {
  const weight = included.reduce((s, p) => s + Math.max(0, p.value), 0)
  const rows = included.map((p, i) => {
    const num = total * Math.max(0, p.value)
    return { i, memberId: p.memberId, amount: Math.floor(num / weight), frac: num % weight, remainder: 0 }
  })
  let left = total - rows.reduce((s, r) => s + r.amount, 0)
  const order = [...rows].sort((a, b) => b.frac - a.frac || a.i - b.i)
  for (const row of order) {
    if (left <= 0) break
    row.amount += 1
    row.remainder += 1
    left -= 1
  }
  return rows.map(({ memberId, amount, remainder }) => ({ memberId, amount, remainder }))
}
