import type { CSSProperties, ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { formatMoney, tallyUnit } from '../money'
import { Figure } from './icons'

export type Tone = 'favor' | 'against' | 'even'

export function toneOf(amount: number): Tone {
  return amount > 0 ? 'favor' : amount < 0 ? 'against' : 'even'
}

export function Amount({ minor, currency, signed, tone }: { minor: number; currency: string; signed?: boolean; tone?: Tone }) {
  return <span className={`amount${tone ? ` amount-${tone}` : ''}`}>{formatMoney(minor, currency, { signed })}</span>
}

// The User's own Balance in words: "Te deben", "Debes", "Estás a mano".
export function useBalanceLabel() {
  const { t } = useTranslation()
  return (amount: number) => (amount > 0 ? t('balance.owedToYou') : amount < 0 ? t('balance.youOwe') : t('balance.settled'))
}

// Isotype counting: every token stands for the same amount, so a Balance is read
// by counting, and a part of a token is cut, never scaled.
export function Tally({ amount, unit, align = 'start' }: { amount: number; unit: number; align?: 'start' | 'end' }) {
  const count = Math.abs(amount) / unit
  const full = Math.floor(count)
  const part = count - full
  const tokens: ReactNode[] = []
  for (let i = 0; i < full; i++) tokens.push(<i key={i} className="token" style={{ '--i': i } as CSSProperties} />)
  if (part >= 0.05) {
    tokens.push(
      <i key="part" className="token-part" style={{ '--i': full, '--f': part } as CSSProperties}>
        <i className="token" />
      </i>,
    )
  }
  return (
    <span className={`tally tally-${align}`} aria-hidden="true">
      {tokens}
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

// Full-strength color fields, one per currency: blue when owed, red when owing.
// Balances in different currencies are never added together.
export function BalanceBands({
  balances,
  footer,
  settledTitle,
  settledBody,
}: {
  balances: ReadonlyMap<string, number>
  footer?: ReactNode
  settledTitle?: string
  settledBody?: string
}) {
  const { t } = useTranslation()
  const label = useBalanceLabel()
  const entries = [...balances]
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
      {entries.map(([currency, amount], i) => {
        const unit = tallyUnit(amount, currency)
        return (
          <div key={currency} className={`band band-${toneOf(amount)}`}>
            <div className="band-inner">
              <p className="band-label">{label(amount)}</p>
              <p className={`band-amount${compact ? ' band-amount-compact' : ''}`}>{formatMoney(Math.abs(amount), currency)}</p>
              <div className="band-tally">
                <Tally amount={amount} unit={unit} />
                <TallyLegend unit={unit} currency={currency} />
              </div>
            </div>
            {footer && i === entries.length - 1 && <div className="band-inner band-footer">{footer}</div>}
          </div>
        )
      })}
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
