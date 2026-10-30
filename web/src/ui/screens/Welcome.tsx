import { useId, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import type { User } from '../../api/client'
import { currencyOptions, session, suggestCurrency } from '../../session'
import { useOnline } from '../components/Shell'
import { SignInHero } from './SignIn'

// A User who signs in with a code for the first time has no name. Before
// anything else the app asks for it, and for a Preferred currency, proposing
// the one of the browser's region (docs/reglas-de-negocio.md).
export function Welcome({ user }: { user: User }) {
  const { t } = useTranslation()
  const ids = useId()
  const online = useOnline()
  const [name, setName] = useState(user.name)
  const [currency, setCurrency] = useState(() => suggestCurrency(navigator.language) ?? user.preferredCurrency)
  const [nameError, setNameError] = useState<string>()
  const [error, setError] = useState<string>()
  const [saving, setSaving] = useState(false)

  const save = async () => {
    const trimmed = name.trim()
    if (!trimmed) {
      setNameError(t('welcome.nameRequired'))
      return
    }
    setSaving(true)
    setError(undefined)
    const result = await session.saveProfile({ name: trimmed, preferredCurrency: currency })
    // Saved: the app moves on by itself.
    if (result.ok) return
    setSaving(false)
    setError(result.error.kind === 'offline' ? t('common.onlineOnly') : t('welcome.failed'))
  }

  const signOut = async () => {
    const result = await session.signOut()
    if (!result.ok) setError(result.error.kind === 'offline' ? t('common.onlineOnly') : t('account.signOutFailed'))
  }

  return (
    <div className="signin">
      <SignInHero art={false} />
      <main className="signin-body">
        <form
          className="form"
          noValidate
          onSubmit={(e) => {
            e.preventDefault()
            void save()
          }}
        >
          <h1 className="page-title">{t('welcome.title')}</h1>
          <p className="lead">{t('welcome.lead')}</p>
          <div className="field">
            <label htmlFor={`${ids}-name`}>{t('welcome.name')}</label>
            <input
              id={`${ids}-name`}
              className="input"
              autoComplete="name"
              autoCapitalize="words"
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
              {t('welcome.currencyHint')}
            </p>
          </div>
          {(error || !online) && (
            <p className="notice notice-problem" role="alert">
              {error ?? t('common.onlineOnly')}
            </p>
          )}
          <button type="submit" className="button button-primary button-block" disabled={saving || !online}>
            {saving ? t('welcome.saving') : t('welcome.start')}
          </button>
          <div className="button-row button-row-split">
            <p className="hint">{t('welcome.signedInAs', { email: user.email })}</p>
            <button type="button" className="button button-quiet button-small" disabled={!online} onClick={() => void signOut()}>
              {t('signIn.changeEmail')}
            </button>
          </div>
        </form>
      </main>
    </div>
  )
}

// Every currency a User can prefer, by code with its name: "MXN · peso mexicano".
export function CurrencySelect({ id, value, onChange, describedBy }: { id: string; value: string; onChange: (currency: string) => void; describedBy?: string }) {
  const { i18n } = useTranslation()
  const names = useMemo(() => new Intl.DisplayNames([i18n.language], { type: 'currency' }), [i18n.language])
  // A currency set before this list existed still shows, rather than a blank.
  const options = currencyOptions.includes(value) ? currencyOptions : [value, ...currencyOptions]
  return (
    <select id={id} className="input" value={value} onChange={(e) => onChange(e.target.value)} aria-describedby={describedBy}>
      {options.map((code) => (
        <option key={code} value={code}>
          {`${code} · ${names.of(code) ?? code}`}
        </option>
      ))}
    </select>
  )
}
