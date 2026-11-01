import { tallyUnit } from '../money'

// How tokens move (DESIGN.md, Movimiento): each one takes 0.5s, 40ms after the
// one before it. styles.css has the same numbers.
const tokenMs = 500
const staggerMs = 40

// How long a row takes to empty `count` tokens, the last one first.
export function emptyingMs(count: number): number {
  return count > 0 ? tokenMs + staggerMs * (count - 1) : 0
}

// The token at each place of a row: 1 when whole, less when it is the cut one.
export function tokensOf(amount: number, unit: number): number[] {
  const count = Math.abs(amount) / unit
  const full = Math.floor(count)
  const part = count - full
  const tokens: number[] = Array.from({ length: full }, () => 1)
  if (part >= 0.05) tokens.push(part)
  return tokens
}

// One band of Balance: a currency, its amount and what each token stands for.
// A total across ledgers comes as a bigint, since it can go past the safe
// integers; the unit only needs its size, which a number is close enough for.
export interface Band {
  currency: string
  amount: number | bigint
  unit: number
}

export function bandsOf(balances: ReadonlyMap<string, number | bigint>): Band[] {
  return [...balances].map(([currency, amount]) => ({ currency, amount, unit: tallyUnit(Number(amount), currency) }))
}

// How long the bands that reached zero take to empty: as long as their longest row.
export function settlingMs(shown: readonly Band[], bands: readonly Band[]): number {
  const rows = withSettling(shown, bands).settling.map(({ amount, unit }) => tokensOf(Number(amount), unit).length)
  return emptyingMs(Math.max(0, ...rows))
}

// The bands to show, given those on screen and those there are now. A currency
// whose Balance just reached zero keeps its band, in its place, at zero and with
// the unit its tokens had, until its row empties. New currencies go last.
export function withSettling(shown: readonly Band[], bands: readonly Band[]): { bands: Band[]; settling: Band[] } {
  const current = new Map(bands.map((band) => [band.currency, band]))
  const settling = shown.filter(({ currency }) => !current.has(currency))
  if (settling.length === 0) return { bands: [...bands], settling }
  const kept = shown.map((band) => current.get(band.currency) ?? { ...band, amount: 0 })
  const added = bands.filter(({ currency }) => !shown.some((band) => band.currency === currency))
  return { bands: [...kept, ...added], settling }
}
