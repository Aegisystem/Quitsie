import { liveQuery, type Observable } from 'dexie'
import { useEffect, useState, useSyncExternalStore } from 'react'
import { api } from '../../api/client'
import { engine, replica, session } from '../../session'
import { createActions } from './actions'
import { watchQuery } from './watch'

export { type ActionResult, type RecordKind } from './actions'
export { type ActivityLine, describeActivity } from './activity'
export { balanceProof, friendOf, ledgerName, myBalance, myMember, myTotals, relabelAmount } from './ledgers'

// The device's one replica and sync engine live in the session, which starts,
// stops and empties them.
export { engine, replica }
export const actions = createActions({ api, engine })

// The last value of each query, so that coming back to a screen draws it at
// once instead of an empty frame while IndexedDB answers. It belongs to whoever
// is signed in.
const lastValues = new Map<string, unknown>()
session.subscribe(() => {
  if (session.getState().status !== 'signedIn') lastValues.clear()
})

// Renders every value of a live query and unsubscribes when the screen goes
// away. `key` names the query: the same key is the same query. `loaded` tells a
// query that has not answered yet from one that answered undefined. A query
// that fails is tried again (watchQuery), and the diagnostics log names it by
// the part of the key before any id.
export function useLive<T>(key: string, query: () => Observable<T>): [value: T | undefined, loaded: boolean] {
  const [latest, setLatest] = useState<{ key: string; value: T }>()

  useEffect(
    () =>
      watchQuery<T>(key.split(':')[0] ?? key, query, (value) => {
        lastValues.set(key, value)
        setLatest({ key, value })
      }),
    // The key stands for the query, which is a new function on every render.
    [key],
  )

  if (latest?.key === key) return [latest.value, true]
  return [lastValues.get(key) as T | undefined, lastValues.has(key)]
}

// Until the first pull, an empty replica says nothing about the User: it does
// not mean they have no Groups. `undefined` while that is not known yet.
export function useReplicaFilled(): boolean | undefined {
  const [cursor, loaded] = useLive('cursor', () => liveQuery(() => replica.meta.get('cursor')))
  return loaded ? cursor !== undefined : undefined
}

export function useSyncStatus() {
  return useSyncExternalStore(engine.subscribe, engine.getStatus)
}
