-- name: CreateUser :exec
INSERT INTO users (id, email, name, preferred_currency, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = ? AND deleted_at IS NULL;

-- name: GetUserByGoogleSub :one
SELECT * FROM users
WHERE google_sub = sqlc.arg(google_sub) AND deleted_at IS NULL;

-- name: SetUserGoogleSub :exec
UPDATE users
SET google_sub = sqlc.arg(google_sub)
WHERE id = sqlc.arg(id);
