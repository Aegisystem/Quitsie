-- name: GetKnownDevice :one
SELECT * FROM known_devices
WHERE token_hash = ?;

-- name: CreateKnownDevice :exec
INSERT INTO known_devices (token_hash, user_id, created_at, last_used_at)
VALUES (?, ?, ?, ?);

-- name: TouchKnownDevice :exec
UPDATE known_devices
SET last_used_at = ?
WHERE token_hash = ?;

-- name: DeleteStaleKnownDevices :exec
-- Keeps the User's most recently used devices that are still alive.
DELETE FROM known_devices
WHERE known_devices.user_id = sqlc.arg(user_id)
  AND (known_devices.last_used_at <= sqlc.arg(used_before)
       OR known_devices.token_hash NOT IN (
           SELECT kept.token_hash FROM known_devices AS kept
           WHERE kept.user_id = sqlc.arg(user_id)
           ORDER BY kept.last_used_at DESC, kept.token_hash
           LIMIT sqlc.arg(keep)));

-- name: DeleteKnownDevicesOfUser :exec
DELETE FROM known_devices
WHERE user_id = ?;
