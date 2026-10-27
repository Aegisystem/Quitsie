import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import { Figure, Icon } from '../components/icons'
import { Screen, toast } from '../components/Shell'
import { demo, ledgerById, useDemo } from '../demo/store'
import { href } from '../router'

export function Account() {
  const { t } = useTranslation()
  const s = useDemo()
  const ids = useId()

  return (
    <Screen tab="account">
      <h1 className="page-title">{t('account.title')}</h1>
      <div className="profile">
        <Figure size={48} />
        <div>
          <p className="profile-name">{s.me.name}</p>
          <p className="row-sub">{s.me.email}</p>
        </div>
      </div>

      <div className="field">
        <label htmlFor={`${ids}-currency`}>{t('account.preferredCurrency')}</label>
        <select
          id={`${ids}-currency`}
          className="input"
          value={s.me.preferredCurrency}
          onChange={(e) => demo.setPreferredCurrency(e.target.value)}
          aria-describedby={`${ids}-currency-hint`}
        >
          {['EUR', 'USD', 'MXN', 'COP', 'CLP', 'ARS', 'PEN', 'GBP'].map((c) => (
            <option key={c}>{c}</option>
          ))}
        </select>
        <p className="hint" id={`${ids}-currency-hint`}>
          {t('account.preferredCurrencyHint')}
        </p>
      </div>

      <button type="button" className="button button-secondary" onClick={() => demo.signOut()}>
        {t('account.signOut')}
      </button>

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
