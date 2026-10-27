import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { api, type Health } from '../api/client'

async function fetchHealth(): Promise<Health> {
  const { data, error, response } = await api.GET('/health')
  if (data) return data
  // A 503 still carries a Health body that says what is down.
  if (response.status === 503 && error) return error
  throw new Error(`HTTP ${response.status}`)
}

export function App() {
  const { t } = useTranslation()
  const health = useQuery({ queryKey: ['health'], queryFn: fetchHealth, retry: false })

  let tone: 'pending' | 'ok' | 'down' = 'pending'
  let message = t('health.checking')
  if (health.isError) {
    tone = 'down'
    message = t('health.unreachable')
  } else if (health.data) {
    tone = health.data.status === 'ok' ? 'ok' : 'down'
    message = health.data.status === 'ok' ? t('health.ok') : t('health.unavailable')
  }

  return (
    <main className="shell">
      <header>
        <h1>{t('app.name')}</h1>
        <p className="tagline">{t('app.tagline')}</p>
      </header>

      <section className="card" aria-labelledby="health-title">
        <h2 id="health-title">{t('health.title')}</h2>
        <p className={`status status-${tone}`} role="status">
          <span className="dot" aria-hidden="true" />
          {message}
        </p>
        {health.data && (
          <dl className="details">
            <dt>{t('health.database')}</dt>
            <dd>{health.data.database === 'ok' ? t('health.databaseOk') : t('health.databaseUnreachable')}</dd>
            <dt>{t('health.version')}</dt>
            <dd>
              <code>{health.data.version}</code>
            </dd>
          </dl>
        )}
        {tone === 'down' && (
          <button type="button" onClick={() => void health.refetch()} disabled={health.isFetching}>
            {t('health.retry')}
          </button>
        )}
      </section>
    </main>
  )
}
