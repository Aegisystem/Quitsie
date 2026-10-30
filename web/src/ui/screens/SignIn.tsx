import { useEffect, useId, useRef, useState, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'
import { googleSignInUrl, session, type CallError, type SignedOutNotice } from '../../session'
import { GoogleMark, Icon, Mark } from '../components/icons'
import { useOnline } from '../components/Shell'

// The server allows 5 attempts per code, and a new code replaces the old one.
const MAX_ATTEMPTS = 5
const RESEND_SECONDS = 30

// Sign in with a 6-digit code or Google; no magic links, because an installed
// PWA on iOS does not share its session with Safari (ADR 0005).
export function SignIn({ notice }: { notice?: SignedOutNotice }) {
  const { t } = useTranslation()
  const ids = useId()
  const online = useOnline()
  const [step, setStep] = useState<'email' | 'code'>('email')
  const [email, setEmail] = useState('')
  const [emailError, setEmailError] = useState<string>()
  const [code, setCode] = useState('')
  const [attemptsLeft, setAttemptsLeft] = useState(MAX_ATTEMPTS)
  const [codeError, setCodeError] = useState<string>()
  const [resendIn, setResendIn] = useState(RESEND_SECONDS)
  const [resent, setResent] = useState(false)
  const [sending, setSending] = useState(false)
  const [checking, setChecking] = useState(false)
  const codeRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    if (step !== 'code' || resendIn <= 0) return
    const id = window.setTimeout(() => setResendIn((n) => n - 1), 1000)
    return () => window.clearTimeout(id)
  }, [step, resendIn])

  const focusCode = () => window.setTimeout(() => codeRef.current?.focus(), 50)

  const callFailed = (error: CallError) => {
    if (error.kind === 'offline') return t('signIn.offline')
    if (error.code === 'rate_limited') {
      return error.retryAfter ? t('signIn.rateLimitedFor', { count: Math.ceil(error.retryAfter / 60) }) : t('signIn.rateLimited')
    }
    if (error.code === 'validation_failed') return t('signIn.invalidEmail')
    return t('signIn.failed')
  }

  const sendCode = async (resend: boolean) => {
    const address = email.trim()
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(address)) {
      setEmailError(t('signIn.invalidEmail'))
      return
    }
    setSending(true)
    const result = await session.requestCode(address)
    setSending(false)
    if (!result.ok) {
      if (resend) setCodeError(callFailed(result.error))
      else setEmailError(callFailed(result.error))
      return
    }
    setEmailError(undefined)
    setCodeError(undefined)
    setStep('code')
    setCode('')
    setResendIn(RESEND_SECONDS)
    setAttemptsLeft(MAX_ATTEMPTS)
    setResent(resend)
    focusCode()
  }

  const verify = async (value: string) => {
    if (value.length !== 6 || checking) return
    setChecking(true)
    const result = await session.verifyCode(email.trim(), value)
    // Signed in: the app swaps this screen for the next one.
    if (result.ok) return
    setChecking(false)
    setCode('')
    const { error } = result
    const reason = error.kind === 'problem' ? error.code : undefined
    if (reason === 'login_code_invalid') {
      const left = Math.max(attemptsLeft - 1, 0)
      setAttemptsLeft(left)
      setCodeError(left > 0 ? t('signIn.wrongCode', { count: left }) : t('signIn.tooManyAttempts'))
    } else if (reason === 'too_many_attempts') {
      setAttemptsLeft(0)
      setCodeError(t('signIn.tooManyAttempts'))
    } else if (reason === 'login_code_expired') {
      setAttemptsLeft(0)
      setCodeError(t('signIn.codeExpired'))
    } else {
      setCodeError(callFailed(error))
    }
    focusCode()
  }

  // Google comes back to the route the User was on, an invite for example.
  const signInWithGoogle = () => {
    const route = location.hash.replace(/^#\/?$/, '')
    location.assign(googleSignInUrl(route ? `/${route}` : undefined))
  }

  const banner = !online ? 'offline' : notice
  const codeUsable = attemptsLeft > 0

  return (
    <div className="signin">
      <SignInHero />

      <main className="signin-body">
        {step === 'email' ? (
          <form
            className="form"
            noValidate
            onSubmit={(e) => {
              e.preventDefault()
              void sendCode(false)
            }}
          >
            <h1 className="page-title">{t('signIn.title')}</h1>
            {banner && (
              <p className={`notice${banner === 'googleFailed' ? ' notice-problem' : ''}`} role={banner === 'googleFailed' ? 'alert' : 'status'}>
                {t(`signIn.${banner}`)}
              </p>
            )}
            <p className="lead">{t('signIn.lead')}</p>
            <div className="field">
              <label htmlFor={`${ids}-email`}>{t('signIn.email')}</label>
              <input
                id={`${ids}-email`}
                className="input"
                type="email"
                inputMode="email"
                autoComplete="email"
                placeholder={t('signIn.emailPlaceholder')}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                aria-invalid={!!emailError}
                aria-describedby={emailError ? `${ids}-email-error` : `${ids}-email-hint`}
              />
              {emailError ? (
                <p className="field-error" id={`${ids}-email-error`} role="alert">
                  {emailError}
                </p>
              ) : (
                <p className="hint" id={`${ids}-email-hint`}>
                  {t('signIn.noPasswords')}
                </p>
              )}
            </div>
            <button type="submit" className="button button-primary button-block" disabled={sending || !online}>
              {sending ? t('signIn.sending') : t('signIn.sendCode')}
            </button>
            <p className="divider">
              <span>{t('signIn.or')}</span>
            </p>
            <button type="button" className="button button-secondary button-block" disabled={!online} onClick={signInWithGoogle}>
              <GoogleMark />
              {t('signIn.google')}
            </button>
          </form>
        ) : (
          <form
            className="form"
            noValidate
            onSubmit={(e) => {
              e.preventDefault()
              void verify(code)
            }}
          >
            <h1 className="page-title">{t('signIn.codeTitle')}</h1>
            <p className="lead">{t('signIn.codeBody', { email: email.trim() })}</p>
            <div className="field">
              <label htmlFor={`${ids}-code`}>{t('signIn.code')}</label>
              <div className={`code-input${codeError ? ' has-error' : ''}`}>
                <input
                  ref={codeRef}
                  id={`${ids}-code`}
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  pattern="[0-9]*"
                  maxLength={6}
                  value={code}
                  disabled={!codeUsable || checking}
                  aria-invalid={!!codeError}
                  aria-describedby={codeError ? `${ids}-code-error` : `${ids}-code-hint`}
                  onChange={(e) => {
                    const digits = e.target.value.replace(/\D/g, '').slice(0, 6)
                    setCode(digits)
                    if (digits.length > 0) setCodeError(undefined)
                    if (digits.length === 6) void verify(digits)
                  }}
                />
                <span className="code-cells" aria-hidden="true">
                  {Array.from({ length: 6 }, (_, i) => (
                    <span key={i} className={`code-cell${i === code.length ? ' is-current' : ''}${code[i] ? ' is-filled' : ''}`}>
                      {code[i] ?? ''}
                    </span>
                  ))}
                </span>
              </div>
              {codeError ? (
                <p className="field-error" id={`${ids}-code-error`} role="alert">
                  {codeError}
                </p>
              ) : (
                <p className="hint" id={`${ids}-code-hint`} role="status">
                  {resent ? t('signIn.resent') : t('signIn.codeHint')}
                </p>
              )}
            </div>
            <button type="submit" className="button button-primary button-block" disabled={code.length !== 6 || !codeUsable || checking}>
              {checking ? t('signIn.checking') : t('signIn.enter')}
            </button>
            <div className="button-row button-row-split">
              <button type="button" className="button button-quiet button-small" disabled={resendIn > 0 || sending || !online} onClick={() => void sendCode(true)}>
                <Icon name="refresh" size={18} />
                {sending ? t('signIn.sending') : resendIn > 0 ? t('signIn.resendIn', { seconds: resendIn }) : t('signIn.resend')}
              </button>
              <button
                type="button"
                className="button button-quiet button-small"
                onClick={() => {
                  setStep('email')
                  setCodeError(undefined)
                }}
              >
                {t('signIn.changeEmail')}
              </button>
            </div>
          </form>
        )}
      </main>
    </div>
  )
}

// The ink header of the screens before the app: sign-in and welcome.
export function SignInHero({ art = true }: { art?: boolean }) {
  const { t } = useTranslation()
  return (
    <header className="signin-hero">
      <div className="signin-hero-inner">
        <p className="signin-wordmark">
          <Mark size={44} />
          <span>{t('app.name')}</span>
        </p>
        <p className="signin-tagline">{t('app.tagline')}</p>
        {art && (
          <div className="signin-art" aria-hidden="true">
            <span className="signin-tally signin-tally-favor">
              {Array.from({ length: 6 }, (_, i) => (
                <i key={i} className="token" style={{ '--i': i } as CSSProperties} />
              ))}
            </span>
            <span className="signin-equals">=</span>
            <span className="signin-tally signin-tally-against">
              {Array.from({ length: 6 }, (_, i) => (
                <i key={i} className="token" style={{ '--i': i + 6 } as CSSProperties} />
              ))}
            </span>
          </div>
        )}
      </div>
    </header>
  )
}
