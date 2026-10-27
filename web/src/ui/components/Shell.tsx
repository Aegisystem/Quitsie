import { useEffect, useState, useSyncExternalStore, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { demo, unsyncedCount, useDemo } from '../demo/store'
import { href, type Route } from '../router'
import { Icon, Mark, type IconName } from './icons'

// Online means the network is up and the demo is not simulating an outage.
function subscribeNetwork(listener: () => void) {
  window.addEventListener('online', listener)
  window.addEventListener('offline', listener)
  return () => {
    window.removeEventListener('online', listener)
    window.removeEventListener('offline', listener)
  }
}

export function useOnline(): boolean {
  const { simulateOffline } = useDemo()
  const network = useSyncExternalStore(subscribeNetwork, () => navigator.onLine)
  return network && !simulateOffline
}

// Toasts

let toastState: { id: number; text: string } | null = null
const toastListeners = new Set<() => void>()
let toastTimer: number | undefined

export function toast(text: string) {
  toastState = { id: (toastState?.id ?? 0) + 1, text }
  for (const l of toastListeners) l()
  window.clearTimeout(toastTimer)
  toastTimer = window.setTimeout(() => {
    toastState = null
    for (const l of toastListeners) l()
  }, 3600)
}

function Toaster() {
  const current = useSyncExternalStore(
    (l) => {
      toastListeners.add(l)
      return () => toastListeners.delete(l)
    },
    () => toastState,
  )
  return (
    <div className="toaster" role="status" aria-live="polite">
      {current && (
        <p key={current.id} className="toast">
          {current.text}
        </p>
      )}
    </div>
  )
}

const tabs: { route: Route; name: 'home' | 'activity' | 'account'; icon: IconName }[] = [
  { route: { name: 'home' }, name: 'home', icon: 'home' },
  { route: { name: 'activity' }, name: 'activity', icon: 'activity' },
  { route: { name: 'account' }, name: 'account', icon: 'account' },
]

function NavLinks({ current, className }: { current?: string; className: string }) {
  const { t } = useTranslation()
  return (
    <nav className={className} aria-label={t('nav.main')}>
      {tabs.map((tab) => (
        <a key={tab.name} href={href(tab.route)} aria-current={current === tab.name ? 'page' : undefined}>
          <Icon name={tab.icon} />
          <span>{t(`nav.${tab.name}`)}</span>
        </a>
      ))}
    </nav>
  )
}

function ConnectionStatus() {
  const { t } = useTranslation()
  const s = useDemo()
  const online = useOnline()
  const unsynced = unsyncedCount(s)
  const rejected = s.rejected.length
  const count = unsynced + rejected
  const label = [
    rejected ? t('connection.rejectedBadge', { count: rejected }) : '',
    unsynced ? t('connection.unsyncedBadge', { count: unsynced }) : '',
  ]
    .filter(Boolean)
    .join(', ')

  return (
    <div className="status">
      {!online && (
        <span className="status-chip">
          <Icon name="offline" size={18} />
          <span>{t('connection.offline')}</span>
        </span>
      )}
      {online && s.syncing && <span className="status-chip status-chip-quiet">{t('connection.syncing')}</span>}
      <a className="records-button" href={href({ name: 'records' })} aria-label={label ? `${t('nav.records')}: ${label}` : t('nav.records')}>
        <Icon name="tray" />
        {count > 0 && <span className={`records-badge${rejected ? ' records-badge-rejected' : ''}`}>{count}</span>}
      </a>
    </div>
  )
}

export function OfflineBanner() {
  const { t } = useTranslation()
  const online = useOnline()
  if (online) return null
  return (
    <div className="offline-banner" role="status">
      <div className="offline-banner-inner">
        <Icon name="offline" size={20} />
        <p>
          <strong>{t('connection.bannerTitle')}.</strong> {t('connection.bannerBody')}
        </p>
      </div>
    </div>
  )
}

export function Screen({
  title,
  back,
  actions,
  tab,
  dock,
  children,
  wide,
}: {
  title?: string
  back?: Route
  actions?: ReactNode
  tab?: 'home' | 'activity' | 'account'
  dock?: ReactNode
  children: ReactNode
  wide?: boolean
}) {
  const { t } = useTranslation()
  return (
    <div className={`screen${tab ? ' screen-tabbed' : ''}${dock ? ' screen-docked' : ''}`}>
      <header className="topbar">
        <div className="topbar-inner">
          {back ? (
            <a className="topbar-back" href={href(back)} aria-label={t('nav.back')}>
              <Icon name="back" />
            </a>
          ) : (
            <a className="wordmark" href={href({ name: 'home' })}>
              <Mark size={28} />
              <span>{t('app.name')}</span>
            </a>
          )}
          {title && <h1 className="topbar-title">{title}</h1>}
          {!back && <NavLinks current={tab} className="topnav" />}
          <div className="topbar-actions">
            {actions}
            <ConnectionStatus />
          </div>
        </div>
      </header>
      <OfflineBanner />
      <main className={`main${wide ? ' main-wide' : ''}`}>{children}</main>
      {dock && (
        <div className="dock">
          <div className="dock-inner">{dock}</div>
        </div>
      )}
      {tab && <NavLinks current={tab} className="tabbar" />}
      <Toaster />
    </div>
  )
}

// PWA install invitation. Chromium fires beforeinstallprompt; iOS Safari has no
// prompt, so it gets the manual steps.

interface InstallEvent extends Event {
  prompt: () => Promise<void>
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>
}

let deferredInstall: InstallEvent | null = null
const installListeners = new Set<() => void>()
window.addEventListener('beforeinstallprompt', (e) => {
  e.preventDefault()
  deferredInstall = e as InstallEvent
  for (const l of installListeners) l()
})

const DISMISSED = 'quitsie.installDismissed'

function isStandalone() {
  return window.matchMedia('(display-mode: standalone)').matches || (navigator as { standalone?: boolean }).standalone === true
}

function isIOS() {
  return /iphone|ipad|ipod/i.test(navigator.userAgent) || (navigator.platform === 'MacIntel' && navigator.maxTouchPoints > 1)
}

export function InstallPrompt() {
  const { t } = useTranslation()
  const { forceInstallPrompt } = useDemo()
  const [, rerender] = useState(0)
  const [dismissed, setDismissed] = useState(() => {
    try {
      return localStorage.getItem(DISMISSED) === '1'
    } catch {
      return false
    }
  })

  useEffect(() => {
    const l = () => rerender((n) => n + 1)
    installListeners.add(l)
    return () => {
      installListeners.delete(l)
    }
  }, [])

  const ios = isIOS()
  const eligible = !isStandalone() && (deferredInstall !== null || ios)
  if (!forceInstallPrompt && (dismissed || !eligible)) return null

  const dismiss = () => {
    try {
      localStorage.setItem(DISMISSED, '1')
    } catch {
      // Storage blocked: it just comes back next time.
    }
    setDismissed(true)
    demo.setForceInstallPrompt(false)
  }

  const install = async () => {
    if (!deferredInstall) return
    await deferredInstall.prompt()
    await deferredInstall.userChoice
    deferredInstall = null
    dismiss()
  }

  return (
    <section className="install" aria-labelledby="install-title">
      <div className="install-art" aria-hidden="true">
        <Mark size={48} />
      </div>
      <div className="install-body">
        <h2 id="install-title">{t('install.title')}</h2>
        <p>
          {t('install.body')} {!deferredInstall && (ios ? t('install.iosSteps') : t('install.manualSteps'))}
        </p>
        <div className="install-actions">
          {deferredInstall && (
            <button type="button" className="button button-light" onClick={() => void install()}>
              <Icon name="download" size={20} />
              {t('install.install')}
            </button>
          )}
          <button type="button" className="button button-ghost-light" onClick={dismiss}>
            {t('install.later')}
          </button>
        </div>
      </div>
    </section>
  )
}
