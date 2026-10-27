-- name: InsertExpense :execrows
-- Use CreateExpense, which also stores the shares. The id comes from the
-- client, so a resent Expense inserts nothing (ADR 0001).
INSERT INTO expenses (id, ledger_id, description, date, currency, total, category, split_method, created_by, version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO NOTHING;

-- name: InsertPaidShare :exec
INSERT INTO paid_shares (expense_id, member_id, amount)
VALUES (?, ?, ?);

-- name: InsertOwedShare :exec
INSERT INTO owed_shares (expense_id, member_id, amount, split_value)
VALUES (?, ?, ?, ?);

-- name: GetExpense :one
SELECT * FROM expenses
WHERE id = ?;

-- name: ListPaidSharesOfExpense :many
SELECT * FROM paid_shares
WHERE expense_id = ?
ORDER BY member_id;

-- name: ListOwedSharesOfExpense :many
SELECT * FROM owed_shares
WHERE expense_id = ?
ORDER BY member_id;

-- name: UpdateExpenseAtVersion :one
-- Use UpdateExpense, which also replaces the shares.
UPDATE expenses
SET description = sqlc.arg(description),
    date = sqlc.arg(date),
    currency = sqlc.arg(currency),
    total = sqlc.arg(total),
    category = sqlc.narg(category),
    split_method = sqlc.arg(split_method),
    version = sqlc.arg(version)
WHERE id = sqlc.arg(id) AND version = sqlc.arg(expected_version) AND deleted_at IS NULL
RETURNING *;

-- name: DeletePaidSharesOfExpense :exec
DELETE FROM paid_shares
WHERE expense_id = ?;

-- name: DeleteOwedSharesOfExpense :exec
DELETE FROM owed_shares
WHERE expense_id = ?;

-- name: DeleteExpense :one
-- A deleted Expense no longer counts in Balances; its shares stay so it can be restored.
UPDATE expenses
SET deleted_at = ?, version = ?
WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: RestoreExpense :one
UPDATE expenses
SET deleted_at = NULL, version = ?
WHERE id = ? AND deleted_at IS NOT NULL
RETURNING *;
