import { liveQuery, type Observable } from 'dexie'
import type { Member } from '../api/client'
import type { Ledger, Replica } from './db'

export interface LedgerSummary {
  ledger: Ledger
  members: Member[]
}

// Home: every Group and Friendship the User can see.
export function liveHome(replica: Replica): Observable<LedgerSummary[]> {
  return liveQuery(async () => {
    const [ledgers, members] = await Promise.all([replica.ledgers.toArray(), replica.members.toArray()])
    return ledgers.map((ledger) => ({ ledger, members: members.filter((m) => m.ledgerId === ledger.id) }))
  })
}
