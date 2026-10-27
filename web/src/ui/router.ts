import { useSyncExternalStore } from 'react'

// A small hash router: the PWA is served from one index.html and works offline,
// so routes live after the "#" and never reach the server.

export type Route =
  | { name: 'home' }
  | { name: 'activity' }
  | { name: 'account' }
  | { name: 'records' }
  | { name: 'ledger'; ledgerId: string; tab?: string }
  | { name: 'newExpense'; ledgerId?: string; fromRejected?: string }
  | { name: 'settle'; ledgerId: string; from?: string; to?: string; amount?: string; currency?: string }
  | { name: 'inviteLink'; ledgerId: string }
  | { name: 'invite'; token: string }
  | { name: 'notFound' }

function parse(hash: string): Route {
  const [path = '', query = ''] = hash.replace(/^#/, '').split('?')
  const params = new URLSearchParams(query)
  const parts = path.split('/').filter(Boolean).map(decodeURIComponent)
  const opt = (key: string) => params.get(key) ?? undefined

  switch (parts[0]) {
    case undefined:
      return { name: 'home' }
    case 'activity':
      return { name: 'activity' }
    case 'account':
      return { name: 'account' }
    case 'records':
      return { name: 'records' }
    case 'ledgers':
      if (parts[1] && parts[2] === 'invite') return { name: 'inviteLink', ledgerId: parts[1] }
      if (parts[1]) return { name: 'ledger', ledgerId: parts[1], tab: opt('tab') }
      break
    case 'expenses':
      if (parts[1] === 'new') return { name: 'newExpense', ledgerId: opt('ledger'), fromRejected: opt('fix') }
      break
    case 'settle':
      if (opt('ledger')) return { name: 'settle', ledgerId: opt('ledger') ?? '', from: opt('from'), to: opt('to'), amount: opt('amount'), currency: opt('currency') }
      break
    case 'invite':
      if (parts[1]) return { name: 'invite', token: parts[1] }
      break
  }
  return { name: 'notFound' }
}

export function href(route: Route): string {
  const q = (params: Record<string, string | undefined>) => {
    const entries = Object.entries(params).filter((e): e is [string, string] => e[1] !== undefined)
    return entries.length ? `?${new URLSearchParams(entries).toString()}` : ''
  }
  switch (route.name) {
    case 'home':
      return '#/'
    case 'activity':
      return '#/activity'
    case 'account':
      return '#/account'
    case 'records':
      return '#/records'
    case 'ledger':
      return `#/ledgers/${encodeURIComponent(route.ledgerId)}${q({ tab: route.tab })}`
    case 'newExpense':
      return `#/expenses/new${q({ ledger: route.ledgerId, fix: route.fromRejected })}`
    case 'settle':
      return `#/settle${q({ ledger: route.ledgerId, from: route.from, to: route.to, amount: route.amount, currency: route.currency })}`
    case 'inviteLink':
      return `#/ledgers/${encodeURIComponent(route.ledgerId)}/invite`
    case 'invite':
      return `#/invite/${encodeURIComponent(route.token)}`
    case 'notFound':
      return '#/'
  }
}

export function navigate(route: Route, opts: { replace?: boolean } = {}) {
  const target = href(route)
  if (opts.replace) {
    history.replaceState(null, '', target)
    window.dispatchEvent(new HashChangeEvent('hashchange'))
  } else {
    location.hash = target
  }
}

function subscribe(listener: () => void) {
  window.addEventListener('hashchange', listener)
  return () => window.removeEventListener('hashchange', listener)
}

let cachedHash: string | undefined
let cachedRoute: Route = { name: 'home' }

export function useRoute(): Route {
  return useSyncExternalStore(subscribe, () => {
    if (location.hash !== cachedHash) {
      cachedHash = location.hash
      cachedRoute = parse(location.hash)
    }
    return cachedRoute
  })
}
