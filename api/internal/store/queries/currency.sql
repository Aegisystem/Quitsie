-- Use RelabelCurrency, which runs these three together.

-- name: SetGroupCurrency :one
UPDATE ledgers
SET currency = CAST(sqlc.arg(currency) AS TEXT), version = sqlc.arg(version)
WHERE id = sqlc.arg(id) AND kind = 'group' AND deleted_at IS NULL
RETURNING *;

-- name: RelabelExpenses :exec
-- Deleted Expenses too, so restoring one never brings back the old currency.
UPDATE expenses
SET currency = ?, version = ?
WHERE ledger_id = ?;

-- name: RelabelSettlements :exec
UPDATE settlements
SET currency = ?, version = ?
WHERE ledger_id = ?;
