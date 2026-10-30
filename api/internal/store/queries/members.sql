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

-- name: ReactivateMember :one
-- Whoever comes back to a Group they left gets their former member back,
-- with its whole history, instead of a new one.
UPDATE members
SET status = 'active', version = ?
WHERE id = ? AND status = 'former'
RETURNING *;

-- name: GetUserSharingAGroup :one
-- The User other_user_id, if they and user_id have an active Member in the same
-- live Group: the only way to start a Friendship directly with an existing User.
SELECT users.id, users.name FROM users
JOIN members theirs ON theirs.user_id = users.id AND theirs.status = 'active'
JOIN members mine ON mine.ledger_id = theirs.ledger_id AND mine.status = 'active'
JOIN ledgers ON ledgers.id = mine.ledger_id
WHERE users.id = CAST(sqlc.arg(other_user_id) AS TEXT)
  AND mine.user_id = CAST(sqlc.arg(user_id) AS TEXT)
  AND ledgers.kind = 'group'
  AND ledgers.deleted_at IS NULL
LIMIT 1;

-- name: GetFriendshipBetween :one
-- The live Friendship between two Users, if any.
SELECT ledgers.* FROM ledgers
JOIN members mine ON mine.ledger_id = ledgers.id
JOIN members theirs ON theirs.ledger_id = ledgers.id
WHERE mine.user_id = CAST(sqlc.arg(user_id) AS TEXT)
  AND theirs.user_id = CAST(sqlc.arg(other_user_id) AS TEXT)
  AND ledgers.kind = 'friendship'
  AND ledgers.deleted_at IS NULL
LIMIT 1;

-- name: UpdatePendingMember :one
-- Only a pending Member: a linked one is whoever their User is.
UPDATE members
SET name = ?, email = ?, version = ?
WHERE id = ? AND user_id IS NULL AND status = 'active'
RETURNING *;
