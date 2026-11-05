-- name: CreateLoginCode :exec
INSERT INTO login_codes (id, email, code_hash, flow_hash, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetLoginCodeOfFlow :one
SELECT * FROM login_codes
WHERE flow_hash = $1;

-- name: CountLiveLoginCodes :one
SELECT count(*) FROM login_codes
WHERE email = $1 AND expires_at > $2;

-- name: DeleteLoginCodeOfFlow :exec
DELETE FROM login_codes
WHERE flow_hash = $1;

-- name: DeleteLoginCodes :exec
DELETE FROM login_codes
WHERE email = $1;

-- name: CountLoginCodeAttempt :exec
UPDATE login_codes
SET attempts = attempts + 1
WHERE id = $1;

-- name: DeleteExpiredLoginCodes :exec
DELETE FROM login_codes
WHERE expires_at <= $1;

-- name: GetLoginFailures :one
SELECT * FROM login_failures
WHERE email_hash = $1;

-- name: SetLoginFailures :exec
INSERT INTO login_failures (email_hash, failures, resets_at)
VALUES ($1, $2, $3)
ON CONFLICT (email_hash) DO UPDATE
SET failures = excluded.failures, resets_at = excluded.resets_at;

-- name: DeleteEndedLoginFailures :exec
DELETE FROM login_failures
WHERE resets_at <= $1;
