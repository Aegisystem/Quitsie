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
