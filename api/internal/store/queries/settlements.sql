-- name: InsertSettlement :execrows
-- Use CreateSettlement. The id comes from the client, so a resent Settlement
-- inserts nothing (ADR 0001).
INSERT INTO settlements (id, ledger_id, from_member_id, to_member_id, amount, currency, date, created_by, version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (id) DO NOTHING;

-- name: GetSettlement :one
SELECT * FROM settlements
WHERE id = ?;

-- name: DeleteSettlement :one
UPDATE settlements
SET deleted_at = ?, version = ?
WHERE id = ? AND deleted_at IS NULL
RETURNING *;
