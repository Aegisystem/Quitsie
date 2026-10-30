// Package sync implements the sync protocol in docs/arquitectura.md: pulling
// changes since a cursor, pushing batches of UnsyncedRecords, and the online
// edits, deletions and restorations of the same records. It decides which
// ledgers a User can use and which rules apply, and leaves the contract to
// package httpapi.
package sync
