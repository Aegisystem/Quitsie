import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Screen, useOnline } from './components/Shell'
import { demo, useDemo } from './demo/store'
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

export function App() {
  const { signedIn } = useDemo()
  const route = useRoute()
  const online = useOnline()

  // The queue empties when the app opens and when the connection comes back
  // (docs/arquitectura.md).
  const wasOnline = useRef<boolean | null>(null)
  useEffect(() => {
    if (signedIn && online && wasOnline.current !== true) demo.sync()
    wasOnline.current = online
  }, [online, signedIn])

  useEffect(() => {
    window.scrollTo(0, 0)
  }, [route])

  if (!signedIn) return <SignIn />

  switch (route.name) {
    case 'home':
      return <Home />
    case 'activity':
      return <Activity />
    case 'account':
      return <Account />
    case 'records':
      return <Records />
    case 'ledger':
      return <Ledger ledgerId={route.ledgerId} tab={route.tab} />
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
