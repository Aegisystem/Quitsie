import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { session, useSession } from '../session'
import { Screen, useOnline } from './components/Shell'
import { href, screenOf, useRoute } from './router'
import { Account } from './screens/Account'
import { Activity } from './screens/Activity'
import { EditExpense, NewExpense } from './screens/ExpenseForm'
import { Home } from './screens/Home'
import { Invite, InviteLink } from './screens/Invite'
import { Ledger } from './screens/Ledger'
import { LedgerSettings } from './screens/LedgerSettings'
import { NewFriend } from './screens/NewFriend'
import { NewGroup } from './screens/NewGroup'
import { Records } from './screens/Records'
import { SettlementForm } from './screens/SettlementForm'
import { SignIn } from './screens/SignIn'
import { Welcome } from './screens/Welcome'

export function App() {
  const state = useSession()
  const route = useRoute()
  const online = useOnline()

  // Who is signed in is asked when the app opens and when the connection comes
  // back; offline, the app opens with the last known User.
  useEffect(() => {
    if (online || session.getState().status === 'loading') void session.check()
  }, [online])

  // A new screen starts at the top. A part of a form opened over it is the same
  // screen, and the form keeps its own place (ExpenseForm).
  const screen = screenOf(route)
  useEffect(() => {
    window.scrollTo(0, 0)
  }, [screen])

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
    case 'ledgerSettings':
      return <LedgerSettings ledgerId={route.ledgerId} user={state.user} />
    case 'newGroup':
      return <NewGroup user={state.user} />
    case 'newFriend':
      return <NewFriend user={state.user} />
    case 'newExpense':
      return <NewExpense ledgerId={route.ledgerId} fromRejected={route.fromRejected} />
    case 'editExpense':
      return <EditExpense ledgerId={route.ledgerId} expenseId={route.expenseId} />
    case 'settle':
      return <SettlementForm key={`${route.from}-${route.to}-${route.fromRejected}`} {...route} />
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
