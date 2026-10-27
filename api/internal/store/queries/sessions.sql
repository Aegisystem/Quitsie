-- name: CreateSession :exec
INSERT INTO sessions (id, user_id, token_hash, device, created_at, last_used_at)
VALUES (?, ?, ?, ?, ?, ?);
