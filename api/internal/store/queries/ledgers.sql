-- name: CreateLedger :exec
INSERT INTO ledgers (id, kind, name, currency, simplify_debts, version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetLedger :one
SELECT * FROM ledgers
WHERE id = ?;

-- name: UpdateLedger :one
UPDATE ledgers
SET name = ?, simplify_debts = ?, version = ?
WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: DeleteLedger :one
-- A deleted ledger stays as a versioned row so sync can remove it from replicas.
UPDATE ledgers
SET deleted_at = ?, version = ?
WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: SetLedgerInviteLink :one
-- Stores the hash of a new Invite link, replacing the previous one, or revokes it with NULL.
UPDATE ledgers
SET invite_link_hash = ?, version = ?
WHERE id = ? AND deleted_at IS NULL
RETURNING *;

-- name: GetLedgerByInviteLink :one
SELECT * FROM ledgers
WHERE invite_link_hash = CAST(sqlc.arg(invite_link_hash) AS TEXT) AND deleted_at IS NULL;

-- name: ScaleLedgerExpenseTotals :exec
-- Use ScaleLedgerAmounts, which runs the four ScaleLedger* queries together.
UPDATE expenses
SET total = total * sqlc.arg(multiplier) / sqlc.arg(divisor)
WHERE ledger_id = sqlc.arg(ledger_id);

-- name: ScaleLedgerPaidShares :exec
-- Use ScaleLedgerAmounts, which runs the four ScaleLedger* queries together.
UPDATE paid_shares
SET amount = amount * sqlc.arg(multiplier) / sqlc.arg(divisor)
WHERE expense_id IN (SELECT expenses.id FROM expenses WHERE expenses.ledger_id = sqlc.arg(ledger_id));

-- name: ScaleLedgerOwedShares :exec
-- Use ScaleLedgerAmounts, which runs the four ScaleLedger* queries together.
-- An adjustment is an amount too; percentages and weights have no currency.
UPDATE owed_shares
SET amount = amount * sqlc.arg(multiplier) / sqlc.arg(divisor),
    split_value = CASE
        WHEN expense_id IN (SELECT expenses.id FROM expenses WHERE expenses.ledger_id = sqlc.arg(ledger_id) AND split_method = 'adjustment')
        THEN split_value * sqlc.arg(multiplier) / sqlc.arg(divisor)
        ELSE split_value
    END
WHERE expense_id IN (SELECT expenses.id FROM expenses WHERE expenses.ledger_id = sqlc.arg(ledger_id));

-- name: ScaleLedgerSettlements :exec
-- Use ScaleLedgerAmounts, which runs the four ScaleLedger* queries together.
UPDATE settlements
SET amount = amount * sqlc.arg(multiplier) / sqlc.arg(divisor)
WHERE ledger_id = sqlc.arg(ledger_id);
