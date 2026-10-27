-- name: SetGroupCurrency :one
-- Use RelabelCurrency, which runs SetGroupCurrency, RelabelExpenses and RelabelSettlements together.
UPDATE ledgers
SET currency = CAST(sqlc.arg(currency) AS TEXT), version = sqlc.arg(version)
WHERE id = sqlc.arg(id) AND kind = 'group' AND deleted_at IS NULL
RETURNING *;

-- name: RelabelExpenses :exec
-- Use RelabelCurrency, which runs SetGroupCurrency, RelabelExpenses and RelabelSettlements together.
-- Deleted Expenses too, so restoring one never brings back the old currency.
UPDATE expenses
SET currency = ?, version = ?
WHERE ledger_id = ?;

-- name: RelabelSettlements :exec
-- Use RelabelCurrency, which runs SetGroupCurrency, RelabelExpenses and RelabelSettlements together.
UPDATE settlements
SET currency = ?, version = ?
WHERE ledger_id = ?;
