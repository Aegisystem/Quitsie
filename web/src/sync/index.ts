// Sync engine: pulls changes into the local replica and pushes the queue of
// UnsyncedRecords when the app opens, regains focus or reconnects, and after
// every change. No Background Sync: Safari on iOS does not support it.
//
// The app creates one of each, once:
//
//   const replica = openReplica()
//   const engine = createSyncEngine({ replica, api })
//   engine.start()
//
// Screens read with the live queries of ../local, which work the same offline:
//
//   liveHome(replica)                 every Group and Friendship, with Balances
//   liveLedger(replica, ledgerId)     one of them, or undefined
//   liveExpenses(replica, ledgerId)   synced and queued, newest first
//   liveSettlements(replica, ledgerId)
//   liveBalances(replica, ledgerId)   provisional: no rejected records
//   liveDebts(replica, ledgerId)      as the server last computed them
//   liveActivity(replica, ledgerId?)  newest first; every ledger without an id
//   liveRecords(replica)              the inbox: { rejected, unsynced }
//
// and write through the engine. Creating Expenses and Settlements works offline:
//
//   const id = await engine.addExpense(expense)
//   const id = await engine.addSettlement(settlement)
//   await engine.addExpense(fixed, { replacing: rejectedId })
//   await engine.discardRecord(rejectedId)
//
// Anything else goes straight to the API, online only, followed by engine.sync().
// Each live query returns a Dexie Observable: subscribe, render every value it
// emits, and unsubscribe when the screen goes away. getStatus and subscribe fit
// useSyncExternalStore.
export type { Api } from './api'
export { type AddOptions, createSyncEngine, type SyncEngine, type SyncStatus } from './engine'
export { browserEnvironment, type Environment } from './environment'
