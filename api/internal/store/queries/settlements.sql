-- name: InsertSettlement :execrows
-- Use CreateSettlement. The id comes from the client, so a resent Settlement
-- inserts nothing (ADR 0001).
INSERT INTO settlements (id, ledger_id, from_member_id, to_member_id, amount, currency, date, created_by, version, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (id) DO NOTHING;

-- name: GetSettlement :one
SELECT * FROM settlements
WHERE id = $1;

-- name: DeleteSettlement :one
UPDATE settlements
SET deleted_at = $1, version = $2
WHERE id = $3 AND deleted_at IS NULL
RETURNING *;

-- name: RestoreSettlement :one
UPDATE settlements
SET deleted_at = NULL, version = $1
WHERE id = $2 AND deleted_at IS NOT NULL
RETURNING *;
