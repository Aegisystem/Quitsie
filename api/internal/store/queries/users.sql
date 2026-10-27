-- name: CreateUser :exec
INSERT INTO users (id, email, name, preferred_currency, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = ? AND deleted_at IS NULL;
