import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useId, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { Session, User } from '../../api/client'
import { describeDevice, session, type CallResult } from '../../session'
import { Figure, Icon } from '../components/icons'
import { Screen, toast, useOnline } from '../components/Shell'
import { formatRelative } from '../dates'
import { demo, ledgerById, useDemo } from '../demo/store'
import { href, navigate } from '../router'
import { CurrencySelect } from './Welcome'

export function Account({ user }: { user: User }) {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const [signingOut, setSigningOut] = useState(false)

  const signOut = async () => {
    setSigningOut(true)
    const result = await session.signOut()
    if (result.ok) {
      navigate({ name: 'home' }, { replace: true })
      return
    }
    setSigningOut(false)
    toast(result.error.kind === 'offline' ? t('common.onlineOnly') : t('account.signOutFailed'))
  }

  return (
    <Screen tab="account">
      <h1 className="page-title">{t('account.title')}</h1>
      <div className="profile">
        <Figure size={48} />
        <div>
          <p className="profile-name">{user.name}</p>
          <p className="row-sub">{user.email}</p>
        </div>
      </div>

      <ProfileForm user={user} />

      <Sessions userId={user.id} />

      <button
        type="button"
        className="button button-secondary"
        disabled={!online || signingOut}
        title={online ? undefined : t('common.onlineOnly')}
        onClick={() => void signOut()}
      >
        {signingOut ? t('account.signingOut') : t('account.signOut')}
      </button>

      <DeleteAccount />

      <section className="section demo-panel" aria-labelledby="demo-title">
        <h2 id="demo-title" className="section-title">
          {t('account.demoTitle')}
        </h2>
        <p className="hint">{t('account.demoHint')}</p>
        <label className="switch-row">
          <span className="switch-title">{t('account.simulateOffline')}</span>
          <input type="checkbox" role="switch" className="switch" checked={s.simulateOffline} onChange={(e) => demo.setSimulateOffline(e.target.checked)} />
        </label>
        <label className="switch-row">
          <span className="switch-title">{t('account.showInstall')}</span>
          <input type="checkbox" role="switch" className="switch" checked={s.forceInstallPrompt} onChange={(e) => demo.setForceInstallPrompt(e.target.checked)} />
        </label>
        <div className="button-row">
          <a className="button button-secondary button-small" href={href({ name: 'invite', token: ledgerById(s, 'g-cumple')?.inviteToken ?? '' })}>
            {t('account.openInvite')}
          </a>
          <button
            type="button"
            className="button button-quiet button-small"
            onClick={() => {
              demo.reset()
              toast(t('account.resetDone'))
            }}
          >
            <Icon name="restore" size={18} />
            {t('account.reset')}
          </button>
        </div>
      </section>
    </Screen>
  )
}

// Name and Preferred currency. Saving needs the network; the form compares
// with the User the server returned, so it is clean again once saved.
function ProfileForm({ user }: { user: User }) {
  const { t } = useTranslation()
  const ids = useId()
  const online = useOnline()
  const [name, setName] = useState(user.name)
  const [currency, setCurrency] = useState(user.preferredCurrency)
  const [nameError, setNameError] = useState<string>()
  const [saving, setSaving] = useState(false)
  const dirty = name.trim() !== user.name || currency !== user.preferredCurrency

  const save = async () => {
    const trimmed = name.trim()
    if (!trimmed) {
      setNameError(t('account.nameRequired'))
      return
    }
    setSaving(true)
    const result = await session.saveProfile({ name: trimmed, preferredCurrency: currency })
    setSaving(false)
    if (result.ok) {
      setName(result.value.name)
      toast(t('account.saved'))
    } else {
      toast(result.error.kind === 'offline' ? t('common.onlineOnly') : t('account.saveFailed'))
    }
  }

  return (
    <form
      className="form"
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        void save()
      }}
    >
      <div className="field">
        <label htmlFor={`${ids}-name`}>{t('account.name')}</label>
        <input
          id={`${ids}-name`}
          className="input"
          autoComplete="name"
          maxLength={80}
          value={name}
          onChange={(e) => {
            setName(e.target.value)
            if (e.target.value.trim()) setNameError(undefined)
          }}
          aria-invalid={!!nameError}
          aria-describedby={nameError ? `${ids}-name-error` : undefined}
        />
        {nameError && (
          <p className="field-error" id={`${ids}-name-error`} role="alert">
            {nameError}
          </p>
        )}
      </div>
      <div className="field">
        <label htmlFor={`${ids}-currency`}>{t('account.preferredCurrency')}</label>
        <CurrencySelect id={`${ids}-currency`} value={currency} onChange={setCurrency} describedBy={`${ids}-currency-hint`} />
        <p className="hint" id={`${ids}-currency-hint`}>
          {t('account.preferredCurrencyHint')}
        </p>
      </div>
      {dirty && (
        <div className="button-row">
          <button type="submit" className="button button-primary" disabled={saving || !online} title={online ? undefined : t('common.onlineOnly')}>
            {saving ? t('account.saving') : t('account.save')}
          </button>
          <button
            type="button"
            className="button button-quiet"
            onClick={() => {
              setName(user.name)
              setCurrency(user.preferredCurrency)
              setNameError(undefined)
            }}
          >
            {t('common.cancel')}
          </button>
        </div>
      )}
    </form>
  )
}

// Results become thrown errors for TanStack Query, which only needs to know it failed.
function unwrap<T>(result: CallResult<T>): T {
  if (!result.ok) throw new Error(result.error.kind === 'offline' ? 'offline' : `status ${result.error.status}`)
  return result.value
}

// One open session per device, this one first. Closing this one is signing out.
// The query is per User, so whoever signs in next never sees another's devices.
function Sessions({ userId }: { userId: string }) {
  const { t } = useTranslation()
  const online = useOnline()
  const queryClient = useQueryClient()
  const sessions = useQuery({
    queryKey: ['sessions', userId],
    queryFn: async () => unwrap(await session.listSessions()),
    retry: 1,
  })
  const revoke = useMutation({
    mutationFn: async (target: Session) => unwrap(await session.revokeSession(target)),
    onSuccess: (_, target) => {
      toast(t('account.revoked', { device: deviceLabel(t, target.device) }))
      void queryClient.invalidateQueries({ queryKey: ['sessions', userId] })
    },
    onError: () => toast(t('account.revokeFailed')),
  })

  const list = [...(sessions.data ?? [])].sort((a, b) => Number(b.current) - Number(a.current) || b.lastUsedAt.localeCompare(a.lastUsedAt))

  return (
    <section className="section" aria-labelledby="sessions-title">
      <h2 id="sessions-title" className="section-title">
        {t('account.sessionsTitle')}
      </h2>
      <p className="hint">{t('account.sessionsHint')}</p>
      {sessions.isPending ? (
        <p className="empty">{online ? t('account.sessionsLoading') : t('common.onlineOnly')}</p>
      ) : sessions.isError ? (
        <div className="button-row">
          <p className="empty tone-problem">{t('account.sessionsFailed')}</p>
          <button type="button" className="button button-quiet button-small" disabled={!online} onClick={() => void sessions.refetch()}>
            <Icon name="refresh" size={18} />
            {t('account.retry')}
          </button>
        </div>
      ) : (
        <ul className="rows">
          {list.map((s) => {
            const device = deviceLabel(t, s.device)
            return (
              // No glyph: a device is neither a person nor a Group.
              <li key={s.id} className="row" style={{ gridTemplateColumns: 'minmax(0, 1fr) auto' }}>
                <span className="row-main">
                  <span className="row-title">{device}</span>
                  <span className="row-sub">
                    {s.current ? t('account.openedAt', { when: formatRelative(s.createdAt) }) : t('account.lastUsed', { when: formatRelative(s.lastUsedAt) })}
                  </span>
                </span>
                <span className="row-end">
                  {s.current ? (
                    <span className="tag">{t('account.thisDevice')}</span>
                  ) : (
                    <button
                      type="button"
                      className="button button-quiet button-small"
                      disabled={!online || (revoke.isPending && revoke.variables.id === s.id)}
                      title={online ? undefined : t('common.onlineOnly')}
                      aria-label={t('account.revokeLabel', { device })}
                      onClick={() => revoke.mutate(s)}
                    >
                      {t('account.revoke')}
                    </button>
                  )}
                </span>
              </li>
            )
          })}
        </ul>
      )}
    </section>
  )
}

function deviceLabel(t: ReturnType<typeof useTranslation>['t'], device: string): string {
  const d = describeDevice(device)
  if (!d) return device || t('account.unknownDevice')
  if (d.browser && d.os) return t('account.deviceOn', { browser: d.browser, os: d.os })
  return d.browser ?? d.os ?? t('account.unknownDevice')
}

// Deleting the account asks once more. The User's Members and Friends become
// pending again with their name, so nobody else's Balances change.
function DeleteAccount() {
  const { t } = useTranslation()
  const ids = useId()
  const online = useOnline()
  const [confirming, setConfirming] = useState(false)
  const [deleting, setDeleting] = useState(false)

  const remove = async () => {
    setDeleting(true)
    const result = await session.deleteAccount()
    if (result.ok) {
      navigate({ name: 'home' }, { replace: true })
      return
    }
    setDeleting(false)
    toast(result.error.kind === 'offline' ? t('common.onlineOnly') : t('account.deleteFailed'))
  }

  return (
    <div className="danger-zone">
      {confirming ? (
        <div className="confirm" role="group" aria-labelledby={`${ids}-confirm`}>
          <p id={`${ids}-confirm`}>{t('account.deleteConfirm')}</p>
          <p className="hint">{t('account.deleteHint')}</p>
          <div className="button-row">
            <button type="button" className="button button-danger button-small" disabled={deleting || !online} onClick={() => void remove()}>
              {deleting ? t('account.deleting') : t('account.deleteYes')}
            </button>
            <button type="button" className="button button-quiet button-small" disabled={deleting} onClick={() => setConfirming(false)}>
              {t('common.cancel')}
            </button>
          </div>
        </div>
      ) : (
        <>
          <button
            type="button"
            className="button button-quiet"
            disabled={!online}
            title={online ? undefined : t('common.onlineOnly')}
            onClick={() => setConfirming(true)}
          >
            {t('account.deleteAccount')}
          </button>
          <p className="hint">{t('account.deleteHint')}</p>
        </>
      )}
    </div>
  )
}
