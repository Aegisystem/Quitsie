-- name: CreateMember :exec
-- Without a user_id the Member is pending.
INSERT INTO members (id, ledger_id, user_id, name, email, version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ClaimMember :one
-- Links an active pending Member to a User, who inherits its history.
UPDATE members
SET user_id = CAST(sqlc.arg(user_id) AS TEXT), version = sqlc.arg(version)
WHERE id = sqlc.arg(id) AND user_id IS NULL AND status = 'active'
RETURNING *;

-- name: UnclaimMember :one
-- Undoes a Claim made by mistake: the Member is pending again and keeps its name.
UPDATE members
SET user_id = NULL, version = ?
WHERE id = ? AND user_id IS NOT NULL
RETURNING *;

-- name: UnlinkMembersOfUser :exec
-- When a User deletes their account, their Members and Friends are pending
-- again, so nobody else's Balance changes.
UPDATE members
SET user_id = NULL, version = sqlc.arg(version)
WHERE user_id = CAST(sqlc.arg(user_id) AS TEXT);

-- name: ListMembers :many
-- Includes former members, who still appear in past Expenses.
SELECT * FROM members
WHERE ledger_id = ?
ORDER BY created_at, id;

-- name: MarkMemberFormer :one
-- A former member keeps appearing in past Expenses but no longer sees the ledger.
UPDATE members
SET status = 'former', version = ?
WHERE id = ? AND status = 'active'
RETURNING *;

-- name: ListLedgersOfUser :many
SELECT ledgers.* FROM ledgers
JOIN members ON members.ledger_id = ledgers.id
WHERE members.user_id = CAST(sqlc.arg(user_id) AS TEXT)
  AND members.status = 'active'
  AND ledgers.deleted_at IS NULL
ORDER BY ledgers.created_at, ledgers.id;

-- name: GetActiveMemberOfUser :one
-- Every query is limited to the ledgers where the User has an active Member.
SELECT * FROM members
WHERE ledger_id = sqlc.arg(ledger_id)
  AND user_id = CAST(sqlc.arg(user_id) AS TEXT)
  AND status = 'active';
