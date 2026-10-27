-- name: NextVersion :one
-- Call once per write transaction, before writing; stamp every row it touches with the result.
UPDATE version_counter
SET value = value + 1
WHERE id = 1
RETURNING value;
