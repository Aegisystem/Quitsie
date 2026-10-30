-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, token_hash, device, created_at, last_used_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetSessionByTokenHash :one
SELECT sqlc.embed(sessions), sqlc.embed(users)
FROM sessions
JOIN users ON users.id = sessions.user_id
WHERE sessions.token_hash = ? AND users.deleted_at IS NULL;

-- name: TouchSession :exec
UPDATE sessions
SET last_used_at = ?
WHERE id = ?;

-- name: DeleteSession :execrows
DELETE FROM sessions
WHERE id = ? AND user_id = ?;

-- name: ListSessions :many
SELECT * FROM sessions
WHERE user_id = sqlc.arg(user_id) AND last_used_at > sqlc.arg(used_after)
ORDER BY last_used_at DESC, id DESC;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE last_used_at <= ?;

-- name: DeleteSessionsOfUser :exec
DELETE FROM sessions
WHERE user_id = ?;
