import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { session, useSession } from '../session'
import { Screen, useOnline } from './components/Shell'
import { engine } from './data'
import { href, useRoute } from './router'
import { Account } from './screens/Account'
import { Activity } from './screens/Activity'
import { NewExpense } from './screens/ExpenseForm'
import { Home } from './screens/Home'
import { Invite, InviteLink } from './screens/Invite'
import { Ledger } from './screens/Ledger'
import { Records } from './screens/Records'
import { SettlementForm } from './screens/SettlementForm'
import { SignIn } from './screens/SignIn'
import { Welcome } from './screens/Welcome'

export function App() {
  const state = useSession()
  const signedIn = state.status === 'signedIn'
  const route = useRoute()
  const online = useOnline()

  // Who is signed in is asked when the app opens and when the connection comes
  // back; offline, the app opens with the last known User.
  useEffect(() => {
    if (online || session.getState().status === 'loading') void session.check()
  }, [online])

  // The engine runs for whoever is signed in: it syncs now, also right after
  // signing in, and whenever the app comes back or reconnects.
  useEffect(() => {
    if (!signedIn) return
    engine.start()
    return () => engine.stop()
  }, [signedIn])

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [route])

  // The way in. The route stays in the address meanwhile, so an invite opened
  // while signed out is still there after signing in.
  if (state.status === 'loading') return <div className="signin" aria-busy="true" />
  if (state.status === 'signedOut') return <SignIn notice={state.notice} />
  if (state.needsProfile) return <Welcome user={state.user} />

  switch (route.name) {
    case 'home':
      return <Home user={state.user} />
    case 'activity':
      return <Activity user={state.user} />
    case 'account':
      return <Account user={state.user} />
    case 'records':
      return <Records user={state.user} />
    case 'ledger':
      return <Ledger ledgerId={route.ledgerId} tab={route.tab} user={state.user} />
    case 'newExpense':
      return <NewExpense ledgerId={route.ledgerId} fromRejected={route.fromRejected} />
    case 'settle':
      return <SettlementForm key={`${route.from}-${route.to}`} {...route} />
    case 'inviteLink':
      return <InviteLink ledgerId={route.ledgerId} />
    case 'invite':
      return <Invite token={route.token} />
    case 'notFound':
      return <NotFound />
  }
}

function NotFound() {
  const { t } = useTranslation()
  return (
    <Screen back={{ name: 'home' }} title={t('notFound.title')}>
      <p className="lead">{t('notFound.body')}</p>
      <a className="button button-primary" href={href({ name: 'home' })}>
        {t('notFound.home')}
      </a>
    </Screen>
  )
}
