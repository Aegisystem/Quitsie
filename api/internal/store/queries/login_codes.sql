-- name: CreateLoginCode :exec
INSERT INTO login_codes (id, email, code_hash, flow_hash, expires_at, created_at)
VALUES (?, ?, ?, ?, ?, ?);

-- name: GetLoginCodeOfFlow :one
SELECT * FROM login_codes
WHERE flow_hash = ?;

-- name: CountLiveLoginCodes :one
SELECT count(*) FROM login_codes
WHERE email = ? AND expires_at > ?;

-- name: DeleteLoginCodeOfFlow :exec
DELETE FROM login_codes
WHERE flow_hash = ?;

-- name: DeleteLoginCodes :exec
DELETE FROM login_codes
WHERE email = ?;

-- name: CountLoginCodeAttempt :exec
UPDATE login_codes
SET attempts = attempts + 1
WHERE id = ?;

-- name: DeleteExpiredLoginCodes :exec
DELETE FROM login_codes
WHERE expires_at <= ?;

-- name: GetLoginFailures :one
SELECT * FROM login_failures
WHERE email_hash = ?;

-- name: SetLoginFailures :exec
INSERT INTO login_failures (email_hash, failures, resets_at)
VALUES (?, ?, ?)
ON CONFLICT (email_hash) DO UPDATE
SET failures = excluded.failures, resets_at = excluded.resets_at;

-- name: DeleteEndedLoginFailures :exec
DELETE FROM login_failures
WHERE resets_at <= ?;
