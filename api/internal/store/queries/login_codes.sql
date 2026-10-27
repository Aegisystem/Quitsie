-- name: CreateLoginCode :exec
INSERT INTO login_codes (id, email, code_hash, expires_at, created_at)
VALUES (?, ?, ?, ?, ?);

-- name: GetLatestLoginCode :one
SELECT * FROM login_codes
WHERE email = ?
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: DeleteLoginCodes :exec
DELETE FROM login_codes
WHERE email = ?;

-- name: CountLoginCodeAttempt :exec
UPDATE login_codes
SET attempts = attempts + 1
WHERE id = ?;
