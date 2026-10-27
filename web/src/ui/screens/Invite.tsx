import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Amount, toneOf } from '../components/Balance'
import { Figure, GroupGlyph, Icon } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { demo, ledgerBalances, ledgerById, ledgerMembers, ledgerName, memberById, myMemberId, useDemo } from '../demo/store'
import { href, navigate } from '../router'

// Landing for an InviteLink: the visitor Claims a pending member (inheriting its
// history) or joins as a new one. An email match is suggested, never applied.
export function Invite({ token }: { token: string }) {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const ids = useId()
  const ledger = s.ledgers.find((l) => l.inviteToken === token)
  const [choice, setChoice] = useState<string | null>(null)

  if (!ledger) {
    return (
      <Screen back={{ name: 'home' }} title={t('invite.title')}>
        <p className="empty">{t('invite.invalid')}</p>
      </Screen>
    )
  }

  const name = ledgerName(s, ledger)
  const alreadyIn = myMemberId(s, ledger.id)
  const balances = ledgerBalances(s, ledger.id)
  const pending = ledgerMembers(s, ledger.id).filter((m) => !m.userId && m.status === 'active')
  const linked = ledgerMembers(s, ledger.id).filter((m) => m.userId && m.status === 'active')
  const isFriendship = ledger.kind === 'friendship'
  const chosen = choice ? memberById(s, choice) : undefined

  const confirm = () => {
    if (!choice || !online) return
    if (choice === 'new') {
      demo.joinAsNewMember(ledger.id)
      toast(t('invite.joined', { group: name }))
    } else if (chosen) {
      demo.claim(chosen.id)
      toast(t('invite.claimed', { name: chosen.name, group: name }))
    }
    navigate({ name: 'ledger', ledgerId: ledger.id }, { replace: true })
  }

  return (
    <Screen
      back={{ name: 'home' }}
      title={t('invite.title')}
      dock={
        !alreadyIn && (
          <div className="dock-stack">
            {!online && <p className="dock-note">{t('invite.offline')}</p>}
            <button type="button" className="button button-primary button-block" onClick={confirm} aria-disabled={!choice || !online}>
              <Icon name="check" />
              {choice === 'new' || isFriendship ? t('invite.joinCta') : chosen ? t('invite.claimCta', { name: chosen.name }) : t('invite.joinCta')}
            </button>
          </div>
        )
      }
    >
      <div className="invite-head">
        <span className="invite-glyph">{isFriendship ? <Figure size={40} /> : <GroupGlyph size={40} />}</span>
        <h2 className="invite-name">{name}</h2>
        <p className="invite-people" aria-label={linked.map((m) => m.name).join(', ')}>
          {linked.map((m) => (
            <Figure key={m.id} size={22} />
          ))}
          {pending.map((m) => (
            <Figure key={m.id} state="pending" size={22} />
          ))}
          <span>{linked.map((m) => m.name).join(', ')}</span>
        </p>
      </div>

      {alreadyIn ? (
        <div className="notice">
          <p>{t('invite.alreadyMember')}</p>
          <a className="button button-secondary" href={href({ name: 'ledger', ledgerId: ledger.id })}>
            {t('invite.open')}
          </a>
        </div>
      ) : (
        <fieldset className="field">
          <legend className="page-title">{t('invite.whoAreYou')}</legend>
          <p className="hint">{t('invite.claimHint')}</p>
          <div className="claim-list" role="radiogroup">
            {pending.map((m) => {
              const amount = (ledger.kind === 'group' ? balances.get(m.id)?.get(ledger.currency) : undefined) ?? 0
              const matches = !!m.email && m.email === s.me.email
              return (
                <label key={m.id} className={`claim${matches ? ' claim-match' : ''}`}>
                  <input type="radio" name={`${ids}-claim`} checked={choice === m.id} onChange={() => setChoice(m.id)} />
                  <Figure state="pending" size={32} />
                  <span className="claim-main">
                    <span className="claim-name">{m.name}</span>
                    {matches && <span className="claim-hint">{t('invite.emailMatch')}</span>}
                  </span>
                  {ledger.kind === 'group' && amount !== 0 && (
                    <span className="claim-balance">
                      <span className={`tone-${toneOf(amount)}`}>{amount > 0 ? t('balance.memberOwed') : t('balance.memberOwes')}</span>
                      <Amount minor={Math.abs(amount)} currency={ledger.currency} tone={toneOf(amount)} />
                    </span>
                  )}
                </label>
              )
            })}
            {!isFriendship && (
              <label className="claim claim-new">
                <input type="radio" name={`${ids}-claim`} checked={choice === 'new'} onChange={() => setChoice('new')} />
                <span className="claim-plus" aria-hidden="true">
                  <Icon name="plus" />
                </span>
                <span className="claim-main">
                  <span className="claim-name">{t('invite.joinNew')}</span>
                  <span className="claim-hint">{t('invite.joinNewHint')}</span>
                </span>
              </label>
            )}
          </div>
        </fieldset>
      )}
    </Screen>
  )
}

export function InviteLink({ ledgerId }: { ledgerId: string }) {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const ledger = ledgerById(s, ledgerId)
  if (!ledger || !ledger.inviteToken) {
    return (
      <Screen back={{ name: 'home' }} title={t('inviteLink.title')}>
        <p className="empty">{t('ledger.unknown')}</p>
      </Screen>
    )
  }
  const name = ledgerName(s, ledger)
  const url = `${location.origin}${location.pathname}${href({ name: 'invite', token: ledger.inviteToken })}`
  const pending = ledgerMembers(s, ledger.id).filter((m) => !m.userId && m.status === 'active')

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(url)
      toast(t('inviteLink.copied'))
    } catch {
      window.prompt(t('inviteLink.copy'), url)
    }
  }
  const share = async () => {
    try {
      await navigator.share({ title: 'Quitsie', text: t('inviteLink.shareText', { name }), url })
    } catch {
      // Cancelled by the User, nothing to do.
    }
  }

  return (
    <Screen back={{ name: 'ledger', ledgerId: ledger.id }} title={t('inviteLink.title')}>
      <h2 className="page-title">{name}</h2>
      <p className="lead">{ledger.kind === 'group' ? t('inviteLink.body') : t('inviteLink.bodyFriend', { name })}</p>

      <div className="link-box">
        <code>{url}</code>
      </div>
      <div className="button-row">
        <button type="button" className="button button-primary" onClick={() => void copy()} disabled={!online}>
          <Icon name="copy" />
          {t('inviteLink.copy')}
        </button>
        {'share' in navigator && (
          <button type="button" className="button button-secondary" onClick={() => void share()} disabled={!online}>
            <Icon name="share" />
            {t('inviteLink.share')}
          </button>
        )}
      </div>
      {!online && <p className="hint">{t('common.onlineOnly')}</p>}

      <h3 className="section-title">{t('inviteLink.pending')}</h3>
      {pending.length === 0 ? (
        <p className="empty">{t('inviteLink.noPending')}</p>
      ) : (
        <ul className="rows">
          {pending.map((m) => (
            <li key={m.id} className="row">
              <span className="row-glyph">
                <Figure state="pending" />
              </span>
              <span className="row-main">
                <span className="row-title">{m.name}</span>
                <span className="row-sub">{m.email ?? t('ledger.pending')}</span>
              </span>
            </li>
          ))}
        </ul>
      )}

      <div className="danger-zone">
        <button
          type="button"
          className="button button-quiet"
          disabled={!online}
          onClick={() => {
            demo.regenerateInvite(ledger.id)
            toast(t('inviteLink.regenerated'))
          }}
        >
          <Icon name="refresh" size={20} />
          {t('inviteLink.regenerate')}
        </button>
        <p className="hint">{t('inviteLink.regenerateHint')}</p>
      </div>
    </Screen>
  )
}
