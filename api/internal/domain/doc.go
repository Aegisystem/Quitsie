// Package domain validates Expenses and Settlements (sums and Participants),
// relabels them when the Group currency changes, and computes Balances and
// Debts, with or without debt simplification. It does not reimplement the
// split methods: the client computes the OwedShares (ADR 0003).
package domain
