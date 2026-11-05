-- name: GetKnownDevice :one
SELECT * FROM known_devices
WHERE token_hash = $1;

-- name: CreateKnownDevice :exec
INSERT INTO known_devices (token_hash, user_id, created_at, last_used_at)
VALUES ($1, $2, $3, $4);

-- name: TouchKnownDevice :exec
UPDATE known_devices
SET last_used_at = $1
WHERE token_hash = $2;

-- name: DeleteStaleKnownDevices :exec
-- Every User's, also of those who never sign in again.
DELETE FROM known_devices
WHERE last_used_at <= sqlc.arg(used_before);

-- name: DeleteSurplusKnownDevices :exec
-- Keeps the User's most recently used devices.
DELETE FROM known_devices
WHERE known_devices.user_id = sqlc.arg(user_id)
  AND known_devices.token_hash NOT IN (
      SELECT kept.token_hash FROM known_devices AS kept
      WHERE kept.user_id = sqlc.arg(user_id)
      ORDER BY kept.last_used_at DESC, kept.token_hash
      LIMIT sqlc.arg(keep)::BIGINT);

-- name: DeleteKnownDevicesOfUser :exec
DELETE FROM known_devices
WHERE user_id = $1;
