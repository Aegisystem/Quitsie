-- name: CreateLedger :exec
INSERT INTO ledgers (id, kind, name, currency, simplify_debts, version, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: GetLedger :one
SELECT * FROM ledgers
WHERE id = $1;

-- name: CountLedgerRows :one
-- What a ledger weighs in a pull: its records, their shares and its Activities.
SELECT CAST(
    (SELECT count(*) FROM expenses WHERE expenses.ledger_id = sqlc.arg(ledger_id))
    + (SELECT count(*) FROM settlements WHERE settlements.ledger_id = sqlc.arg(ledger_id))
    + (SELECT count(*) FROM activities WHERE activities.ledger_id = sqlc.arg(ledger_id))
    + (SELECT count(*) FROM paid_shares JOIN expenses ON expenses.id = paid_shares.expense_id WHERE expenses.ledger_id = sqlc.arg(ledger_id))
    + (SELECT count(*) FROM owed_shares JOIN expenses ON expenses.id = owed_shares.expense_id WHERE expenses.ledger_id = sqlc.arg(ledger_id))
AS BIGINT) AS row_count;

-- name: UpdateLedger :one
UPDATE ledgers
SET name = $1, simplify_debts = $2, version = $3
WHERE id = $4 AND deleted_at IS NULL
RETURNING *;

-- name: DeleteLedger :one
-- A deleted ledger stays as a versioned row so sync can remove it from replicas.
UPDATE ledgers
SET deleted_at = $1, version = $2
WHERE id = $3 AND deleted_at IS NULL
RETURNING *;

-- name: RegenerateInviteLink :one
-- The Invite link's token is derived from the generation (ADR 0010), so
-- moving to the next one revokes the previous token.
UPDATE ledgers
SET invite_link_generation = invite_link_generation + 1, version = $1
WHERE id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: ScaleLedgerExpenseTotals :exec
-- Use RelabelCurrency, which runs the four ScaleLedger* queries when the decimals change.
UPDATE expenses
SET total = total * sqlc.arg(multiplier) / sqlc.arg(divisor)
WHERE ledger_id = sqlc.arg(ledger_id);

-- name: ScaleLedgerPaidShares :exec
-- Use RelabelCurrency, which runs the four ScaleLedger* queries when the decimals change.
UPDATE paid_shares
SET amount = amount * sqlc.arg(multiplier) / sqlc.arg(divisor)
WHERE expense_id IN (SELECT expenses.id FROM expenses WHERE expenses.ledger_id = sqlc.arg(ledger_id));

-- name: ScaleLedgerOwedShares :exec
-- Use RelabelCurrency, which runs the four ScaleLedger* queries when the decimals change.
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
-- Use RelabelCurrency, which runs the four ScaleLedger* queries when the decimals change.
UPDATE settlements
SET amount = amount * sqlc.arg(multiplier) / sqlc.arg(divisor)
WHERE ledger_id = sqlc.arg(ledger_id);
