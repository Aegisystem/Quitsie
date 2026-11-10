import { useSyncExternalStore } from 'react'

// A small hash router: the PWA is served from one index.html and works offline,
// so routes live after the "#" and never reach the server.

// A part of the Expense form with a screen of its own, opened over the form.
export type ExpensePart = 'paid' | 'split'

export type Route =
  | { name: 'home' }
  | { name: 'activity' }
  | { name: 'account' }
  | { name: 'records' }
  | { name: 'ledger'; ledgerId: string; tab?: string }
  | { name: 'ledgerSettings'; ledgerId: string }
  | { name: 'newGroup' }
  | { name: 'newFriend' }
  | { name: 'newExpense'; ledgerId?: string; fromRejected?: string; part?: ExpensePart }
  | { name: 'editExpense'; ledgerId: string; expenseId: string; part?: ExpensePart }
  | { name: 'settle'; ledgerId: string; from?: string; to?: string; amount?: string; currency?: string; fromRejected?: string }
  | { name: 'inviteLink'; ledgerId: string }
  | { name: 'invite'; token: string }
  | { name: 'notFound' }

export function parseRoute(hash: string): Route {
  const [path = '', query = ''] = hash.replace(/^#/, '').split('?')
  const params = new URLSearchParams(query)
  const parts = path.split('/').filter(Boolean).map(decodeURIComponent)
  const opt = (key: string) => params.get(key) ?? undefined
  const part = (): { part?: ExpensePart } => {
    const value = opt('part')
    return value === 'paid' || value === 'split' ? { part: value } : {}
  }

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
      if (parts[1] && parts[2] === 'settings') return { name: 'ledgerSettings', ledgerId: parts[1] }
      if (parts[1]) return { name: 'ledger', ledgerId: parts[1], tab: opt('tab') }
      break
    case 'groups':
      if (parts[1] === 'new') return { name: 'newGroup' }
      break
    case 'friends':
      if (parts[1] === 'new') return { name: 'newFriend' }
      break
    case 'expenses':
      if (parts[1] === 'new') return { name: 'newExpense', ledgerId: opt('ledger'), fromRejected: opt('fix'), ...part() }
      if (parts[1] && parts[2] === 'edit' && opt('ledger')) return { name: 'editExpense', ledgerId: opt('ledger') ?? '', expenseId: parts[1], ...part() }
      break
    case 'settle':
      if (opt('ledger')) {
        const suggestion = { from: opt('from'), to: opt('to'), amount: opt('amount'), currency: opt('currency') }
        return { name: 'settle', ledgerId: opt('ledger') ?? '', ...suggestion, fromRejected: opt('fix') }
      }
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
    case 'ledgerSettings':
      return `#/ledgers/${encodeURIComponent(route.ledgerId)}/settings`
    case 'newGroup':
      return '#/groups/new'
    case 'newFriend':
      return '#/friends/new'
    case 'newExpense':
      return `#/expenses/new${q({ ledger: route.ledgerId, fix: route.fromRejected, part: route.part })}`
    case 'editExpense':
      return `#/expenses/${encodeURIComponent(route.expenseId)}/edit${q({ ledger: route.ledgerId, part: route.part })}`
    case 'settle':
      return `#/settle${q({ ledger: route.ledgerId, from: route.from, to: route.to, amount: route.amount, currency: route.currency, fix: route.fromRejected })}`
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

// The screen a route shows, leaving out a part of a form opened over it: going
// to a part and back stays on the same screen.
export function screenOf(route: Route): string {
  return href(route.name === 'newExpense' || route.name === 'editExpense' ? { ...route, part: undefined } : route)
}

// Opens a view over the current screen. Going back, with the app's button or
// the browser's, returns to the screen as it was.
export function openOver(route: Route) {
  history.pushState({ over: true }, '', href(route))
  window.dispatchEvent(new HashChangeEvent('hashchange'))
}

// Leaves a view opened with openOver. One reached another way, such as a reload,
// has nothing of the app behind it, so it is replaced by `below`.
export function closeOver(below: Route) {
  if ((history.state as { over?: boolean } | null)?.over) history.back()
  else navigate(below, { replace: true })
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
      cachedRoute = parseRoute(location.hash)
    }
    return cachedRoute
  })
}
