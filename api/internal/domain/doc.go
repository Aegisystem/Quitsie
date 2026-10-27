// Package domain validates Expenses and Settlements (sums and Participants)
// and computes Balances and debt simplification. It does not reimplement the
// split methods: the client computes the OwedShares (ADR 0003).
package domain
