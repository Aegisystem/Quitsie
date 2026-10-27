-- name: NextVersion :one
-- Call once per write transaction, before writing; stamp every row it touches with the result.
UPDATE version_counter
SET value = value + 1
WHERE id = 1
RETURNING value;

-- name: CurrentVersion :one
-- The last version handed out; read in the same transaction as a pull, it is the pull's new cursor.
SELECT value FROM version_counter
WHERE id = 1;
