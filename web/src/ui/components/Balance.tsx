import type { CSSProperties, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { formatMoney } from '../money'
import { Figure } from './icons'
import { type Band, bandsOf, emptyingMs, settlingMs, tokensOf, withSettling } from './tally'
import { useSettled } from './useSettled'

export type Tone = 'favor' | 'against' | 'even'

export function toneOf(amount: number | bigint): Tone {
  return amount > 0 ? 'favor' : amount < 0 ? 'against' : 'even'
}

export function Amount({ minor, currency, signed, tone }: { minor: number | bigint; currency: string; signed?: boolean; tone?: Tone }) {
  return <span className={`amount${tone ? ` amount-${tone}` : ''}`}>{formatMoney(minor, currency, { signed })}</span>
}

// The User's own Balance in words: "Te deben", "Debes", "Estás a mano".
export function useBalanceLabel() {
  const { t } = useTranslation()
  return (amount: number | bigint) => (amount > 0 ? t('balance.owedToYou') : amount < 0 ? t('balance.youOwe') : t('balance.settled'))
}

const rowKey = (row: readonly number[]) => row.join(' ')

// Isotype counting: every token stands for the same amount, so a Balance is read
// by counting, and a part of a token is cut, never scaled. When the Balance goes
// down, the tokens left over leave the way they came in before they go, so the
// row is seen emptying (DESIGN.md).
export function Tally({ amount, unit, align = 'start' }: { amount: number; unit: number; align?: 'start' | 'end' }) {
  const tokens = tokensOf(amount, unit)
  const leaving = useSettled(tokens, rowKey, (shown, now) => emptyingMs(shown.length - now.length)).slice(tokens.length)
  const row = [...tokens, ...leaving]
  return (
    <span className={`tally tally-${align}`} aria-hidden="true">
      {row.map((fraction, i) => {
        const out = i >= tokens.length
        const order = (out ? { '--o': row.length - 1 - i } : { '--i': i }) as Record<string, number>
        return fraction === 1 ? (
          <i key={i} className={out ? 'token token-out' : 'token'} style={order as CSSProperties} />
        ) : (
          <i key={i} className={out ? 'token-part token-out' : 'token-part'} style={{ ...order, '--f': fraction } as CSSProperties}>
            <i className="token" />
          </i>
        )
      })}
    </span>
  )
}

export function TallyLegend({ unit, currency }: { unit: number; currency: string }) {
  const { t } = useTranslation()
  const amount = formatMoney(unit, currency, { trim: true })
  return (
    <span className="tally-legend">
      <span className="sr-only">{t('balance.tallyLabel', { amount })}</span>
      <span aria-hidden="true">
        {t('balance.tallyEach')} <i className="token token-legend" /> = {amount}
      </span>
    </span>
  )
}

const bandsKey = (bands: readonly Band[]) => bands.map(({ currency, amount }) => `${currency} ${amount}`).join(' ')

// The bands to show. Being even is seeing the row empty (DESIGN.md), so a
// currency whose Balance just reached zero keeps its band until it has.
function useBands(balances: ReadonlyMap<string, number | bigint>) {
  const now = bandsOf(balances)
  return withSettling(useSettled(now, bandsKey, settlingMs), now).bands
}

interface BandsProps {
  // A total across ledgers comes as a bigint, since it can go past the safe
  // integers.
  balances: ReadonlyMap<string, number | bigint>
  footer?: ReactNode
  settledTitle?: string
  settledBody?: string
}

// A Saldo that counts UnsyncedRecords looks like any other, so it says so.
export function BalanceBands({ provisional, ...bands }: BandsProps & { provisional?: boolean }) {
  const { t } = useTranslation()
  return (
    <>
      <Bands {...bands} />
      {provisional && (
        <p className="band-note">
          <span className="sync-mark sync-unsynced" aria-hidden="true" />
          {t('balance.provisional')}
        </p>
      )}
    </>
  )
}

// Full-strength color fields, one per currency: blue when owed, red when owing.
// Balances in different currencies are never added together.
function Bands({ balances, footer, settledTitle, settledBody }: BandsProps) {
  const { t } = useTranslation()
  const label = useBalanceLabel()
  const entries = useBands(balances)
  const compact = entries.length > 1

  if (entries.length === 0) {
    return (
      <div className="bands">
        <div className="band band-even">
          <div className="band-inner band-settled">
            <span className="band-figures" aria-hidden="true">
              <Figure size={40} />
              <span className="band-equals">=</span>
              <Figure size={40} />
            </span>
            <div>
              <p className="band-amount band-amount-compact">{settledTitle ?? t('balance.settled')}</p>
              {settledBody && <p className="band-label">{settledBody}</p>}
            </div>
          </div>
          {footer && <div className="band-inner band-footer">{footer}</div>}
        </div>
      </div>
    )
  }

  return (
    <div className="bands">
      {entries.map(({ currency, amount, unit }, i) => (
        <div key={currency} className={`band band-${toneOf(amount)}`}>
          <div className="band-inner">
            <p className="band-label">{label(amount)}</p>
            <p className={`band-amount${compact ? ' band-amount-compact' : ''}`}>{formatMoney(amount < 0 ? -amount : amount, currency)}</p>
            <div className="band-tally">
              {/* Tokens only show the size, which a number is close enough for
                  even past the safe integers. */}
              <Tally amount={Number(amount)} unit={unit} />
              <TallyLegend unit={unit} currency={currency} />
            </div>
          </div>
          {footer && i === entries.length - 1 && <div className="band-inner band-footer">{footer}</div>}
        </div>
      ))}
    </div>
  )
}

export function SyncMark({ state }: { state: 'synced' | 'unsynced' | 'rejected' }) {
  const { t } = useTranslation()
  return (
    <span
      className={`sync-mark sync-${state}`}
      role={state === 'synced' ? undefined : 'img'}
      aria-label={state === 'synced' ? undefined : t(`sync.${state}`)}
      aria-hidden={state === 'synced' ? true : undefined}
      title={t(`sync.${state}`)}
    />
  )
}
