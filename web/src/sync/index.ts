// Sync engine: pulls changes into the local replica and pushes the queue of
// UnsyncedRecords when the app opens, regains focus or reconnects, and after
// every change. No Background Sync: Safari on iOS does not support it.
export { type Api, createSyncEngine, type SyncEngine } from './engine'
