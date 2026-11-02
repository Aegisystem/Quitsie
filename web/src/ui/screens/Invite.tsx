import { useQuery, useQueryClient } from '@tanstack/react-query'
import { type ReactNode, useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Invite as InviteDetails } from '../../api/client'
import { liveLedger } from '../../local'
import { ok } from '../../domain/result'
import { type InviteLinkOwner, ledgers, replica, useSession } from '../../session'
import { useLive } from '../data'
import { Figure, GroupGlyph, Icon } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { href, navigate } from '../router'
import { problemText, useOperation } from './operation'

// Landing for an InviteLink: the visitor Claims a pending member (inheriting its
// history) or joins as a new one. An email match is suggested, never applied.
export function Invite({ token }: { token: string }) {
  const { t } = useTranslation()
  const state = useSession()
  const online = useOnline()
  // What it suggests depends on who asks.
  const userId = state.status === 'signedIn' ? state.user.id : undefined
  const invite = useQuery({ queryKey: ['invite', token, userId], queryFn: () => ledgers.getInvite(token), retry: false })
  const answer = invite.data

  if (!answer) {
    return (
      <Screen back={{ name: 'home' }} title={t('invite.title')}>
        <p className="empty">{online ? t('invite.loading') : t('invite.offlineOpen')}</p>
      </Screen>
    )
  }
  if (!answer.ok) {
    const invalid = answer.error.kind === 'problem' && answer.error.status === 404
    return (
      <Screen back={{ name: 'home' }} title={t('invite.title')}>
        <p className="empty">{invalid ? t('invite.invalid') : problemText(t, answer.error)}</p>
        {!invalid && (
          <button type="button" className="button button-quiet" disabled={!online} onClick={() => void invite.refetch()}>
            <Icon name="refresh" size={20} />
            {t('account.retry')}
          </button>
        )}
      </Screen>
    )
  }
  return <InviteChoice key={token} token={token} details={answer.value} reload={() => void invite.refetch()} />
}

function InviteChoice({ token, details, reload }: { token: string; details: InviteDetails; reload: () => void }) {
  const { t } = useTranslation()
  const ids = useId()
  const online = useOnline()
  const [picked, setPicked] = useState<string>()
  const { busy, error, run, reset } = useOperation()
  const isFriendship = details.kind === 'friendship'
  // Removed from the Group: nothing to pick until a Member readmits them.
  const shut = !!details.joinedMemberId || !!details.removed
  // The email match comes first, but it is only a suggestion: the User picks.
  const pending = [...details.pendingMembers].sort((a, b) => Number(b.id === details.suggestedMemberId) - Number(a.id === details.suggestedMemberId))
  const onlyJoining = !isFriendship && pending.length === 0
  const name = details.groupName ?? details.invitedBy ?? ''
  // A pending Friend's link leads to one place, but claiming it is still a click.
  const choice = picked ?? (onlyJoining ? 'new' : isFriendship ? pending[0]?.id : undefined)
  const chosen = pending.find((member) => member.id === choice)

  const open = (ledgerId: string, text: string) => {
    toast(text)
    navigate({ name: 'ledger', ledgerId }, { replace: true })
  }

  const confirm = () => {
    if (!online || !choice) return
    if (chosen) {
      void run(
        () => ledgers.claimMember(token, chosen.id),
        ({ ledgerId }) => open(ledgerId, isFriendship ? t('invite.claimedFriend', { name }) : t('invite.claimed', { name: chosen.name, group: name })),
        () => {
          setPicked(undefined)
          reload()
        },
      )
    } else {
      void run(
        () => ledgers.joinGroup(token),
        ({ ledgerId }) => open(ledgerId, t('invite.joined', { group: name })),
        reload,
      )
    }
  }

  const pick = (id: string) => {
    reset()
    setPicked(id)
  }

  return (
    <Screen
      back={{ name: 'home' }}
      title={t('invite.title')}
      dock={
        !shut && (
          <div className="dock-stack">
            {!online && <p className="dock-note">{t('invite.offline')}</p>}
            <button type="button" className="button button-primary button-block" onClick={confirm} disabled={busy} aria-disabled={!choice || !online}>
              <Icon name="check" />
              {busy ? t('invite.entering') : chosen ? t('invite.claimCta', { name: chosen.name }) : choice ? t('invite.joinCta') : t('invite.pickFirst')}
            </button>
          </div>
        )
      }
    >
      <div className="invite-head">
        <span className="invite-glyph">{isFriendship ? <Figure size={40} /> : <GroupGlyph size={40} />}</span>
        {/* Once linked, the API may name either of the two Friends as the inviter. */}
        {!(isFriendship && details.joinedMemberId) && <h2 className="invite-name">{name}</h2>}
        {!isFriendship && pending.length > 0 && (
          <p className="invite-people" aria-label={pending.map((member) => member.name).join(', ')}>
            {pending.map((member) => (
              <Figure key={member.id} state="pending" size={22} />
            ))}
            <span>{t('invite.pendingCount', { count: pending.length })}</span>
          </p>
        )}
      </div>

      {details.removed ? (
        <p className="notice" role="status">
          {t('invite.removed')}
        </p>
      ) : details.joinedMemberId ? (
        <div className="notice">
          <p>{isFriendship ? t('invite.alreadyFriend') : t('invite.alreadyMember')}</p>
          <a className="button button-secondary" href={href({ name: 'ledger', ledgerId: details.ledgerId })}>
            {isFriendship ? t('invite.openFriend') : t('invite.open')}
          </a>
        </div>
      ) : isFriendship ? (
        <div>
          <p className="lead">{t('invite.friendAsk', { inviter: name, name: chosen?.name })}</p>
          <p className="hint">{t('invite.claimHint')}</p>
          {chosen?.id === details.suggestedMemberId && <p className="hint">{t('invite.emailMatch')}</p>}
        </div>
      ) : onlyJoining ? (
        <p className="lead">{t('invite.joinOnly')}</p>
      ) : (
        <fieldset className="field">
          <legend className="page-title">{t('invite.whoAreYou')}</legend>
          <p className="hint">{t('invite.claimHint')}</p>
          <div className="claim-list" role="radiogroup">
            {pending.map((member) => {
              const matches = member.id === details.suggestedMemberId
              return (
                <label key={member.id} className={`claim${matches ? ' claim-match' : ''}`}>
                  <input type="radio" name={`${ids}-claim`} checked={picked === member.id} onChange={() => pick(member.id)} />
                  <Figure state="pending" size={32} />
                  <span className="claim-main">
                    <span className="claim-name">{member.name}</span>
                    {matches && <span className="claim-hint">{t('invite.emailMatch')}</span>}
                  </span>
                </label>
              )
            })}
            <label className="claim claim-new">
              <input type="radio" name={`${ids}-claim`} checked={picked === 'new'} onChange={() => pick('new')} />
              <span className="claim-plus" aria-hidden="true">
                <Icon name="plus" />
              </span>
              <span className="claim-main">
                <span className="claim-name">{t('invite.joinNew')}</span>
                <span className="claim-hint">{t('invite.joinNewHint')}</span>
              </span>
            </label>
          </div>
        </fieldset>
      )}

      {error && (
        <p className="notice notice-problem" role="alert">
          {problemText(t, error, {
            invite_link_invalid: t('invite.invalid'),
            member_not_pending: t('invite.taken'),
            not_found: t('invite.taken'),
            already_member: t('invite.alreadyHeld'),
            removed_from_group: t('invite.removed'),
            friendship_exists: t('invite.friendshipExists'),
          })}
        </p>
      )}
    </Screen>
  )
}

export function InviteLink({ ledgerId }: { ledgerId: string }) {
  const { t } = useTranslation()
  const [summary, loaded] = useLive(`ledger:${ledgerId}`, () => liveLedger(replica, ledgerId))

  if (!summary) {
    return (
      <Screen back={{ name: 'home' }} title={t('inviteLink.title')}>
        {loaded && <p className="empty">{t('ledger.unknown')}</p>}
      </Screen>
    )
  }

  const { ledger, members } = summary
  const pending = members.filter((member) => !member.userId && member.status === 'active')
  const name = ledger.kind === 'group' ? ledger.name : (pending[0]?.name ?? '')

  return (
    <Screen back={{ name: 'ledger', ledgerId }} title={t('inviteLink.title')}>
      {name && <h2 className="page-title">{name}</h2>}
      {ledger.kind === 'friendship' && pending.length === 0 ? (
        <p className="empty">{t('inviteLink.friendLinked')}</p>
      ) : (
        <>
          <p className="lead">{ledger.kind === 'group' ? t('inviteLink.body') : t('inviteLink.bodyFriend', { name })}</p>
          <LinkBox ledger={ledger} name={name}>
            {ledger.kind === 'group' && (
              <>
                <h3 className="section-title">{t('inviteLink.pending')}</h3>
                {pending.length === 0 ? (
                  <p className="empty">{t('inviteLink.noPending')}</p>
                ) : (
                  <ul className="rows">
                    {pending.map((member) => (
                      <li key={member.id} className="row">
                        <span className="row-glyph">
                          <Figure state="pending" />
                        </span>
                        <span className="row-main">
                          <span className="row-title">{member.name}</span>
                          <span className="row-sub">{member.email ?? t('ledger.pending')}</span>
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </>
            )}
          </LinkBox>
        </>
      )}
    </Screen>
  )
}

// The link is derived on the server, so it can be shown every time (ADR 0010),
// but only online: the device does not keep it. `children` go between the link
// and regenerating it.
function LinkBox({ ledger, name, children }: { ledger: InviteLinkOwner; name: string; children: ReactNode }) {
  const { t } = useTranslation()
  const online = useOnline()
  const queryClient = useQueryClient()
  const key = ['inviteLink', ledger.id]
  const link = useQuery({ queryKey: key, queryFn: () => ledgers.inviteLink(ledger), retry: false })
  const [confirming, setConfirming] = useState(false)
  const { busy, error, run } = useOperation()
  const offlineTitle = online ? undefined : t('common.onlineOnly')

  const answer = link.data
  const url = answer?.ok ? `${location.origin}${location.pathname}${href({ name: 'invite', token: answer.value.token })}` : undefined

  const copy = async () => {
    if (!url) return
    try {
      await navigator.clipboard.writeText(url)
      toast(t('inviteLink.copied'))
    } catch {
      window.prompt(t('inviteLink.copy'), url)
    }
  }
  const share = async () => {
    if (!url) return
    try {
      await navigator.share({ title: 'Quitsie', text: t('inviteLink.shareText', { name }), url })
    } catch {
      // Cancelled by the User, nothing to do.
    }
  }

  const regenerate = () =>
    void run(
      () => ledgers.regenerateInviteLink(ledger),
      (next) => {
        queryClient.setQueryData(key, ok(next))
        setConfirming(false)
        toast(t('inviteLink.regenerated'))
      },
    )

  return (
    <>
      <div className="link-box" aria-busy={link.isFetching && !url}>
        {url ? <code>{url}</code> : <span>{!online ? t('inviteLink.offline') : answer && !answer.ok ? problemText(t, answer.error) : t('inviteLink.loading')}</span>}
      </div>
      <div className="button-row">
        <button type="button" className="button button-primary" onClick={() => void copy()} disabled={!url}>
          <Icon name="copy" />
          {t('inviteLink.copy')}
        </button>
        {'share' in navigator && (
          <button type="button" className="button button-secondary" onClick={() => void share()} disabled={!url}>
            <Icon name="share" />
            {t('inviteLink.share')}
          </button>
        )}
      </div>

      {children}

      <div className="danger-zone">
        {confirming ? (
          <div className="confirm" role="group" aria-label={t('inviteLink.regenerate')}>
            <p>{t('inviteLink.regenerateConfirm')}</p>
            <p className="hint">{t('inviteLink.regenerateHint')}</p>
            <div className="button-row">
              <button type="button" className="button button-danger button-small" disabled={busy || !online} title={offlineTitle} onClick={regenerate}>
                {t('inviteLink.regenerateYes')}
              </button>
              <button type="button" className="button button-quiet button-small" disabled={busy} onClick={() => setConfirming(false)}>
                {t('common.cancel')}
              </button>
            </div>
          </div>
        ) : (
          <>
            <button type="button" className="button button-quiet" disabled={!online} title={offlineTitle} onClick={() => setConfirming(true)}>
              <Icon name="refresh" size={20} />
              {t('inviteLink.regenerate')}
            </button>
            <p className="hint">{t('inviteLink.regenerateHint')}</p>
          </>
        )}
        {error && (
          <p className="notice notice-problem" role="alert">
            {problemText(t, error, { member_not_pending: t('inviteLink.friendLinked') })}
          </p>
        )}
      </div>
    </>
  )
}
