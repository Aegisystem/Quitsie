-- name: CreateLedger :exec
INSERT INTO ledgers (id, kind, name, currency, simplify_debts, version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: GetLedger :one
SELECT * FROM ledgers
WHERE id = ?;
