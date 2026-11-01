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

-- name: JumpVersion :exec
-- Only RecordRestore moves the counter other than one by one.
UPDATE version_counter
SET value = sqlc.arg(value)
WHERE id = 1;

-- name: CreateRestoreGap :exec
INSERT INTO restore_gaps (restored_version, jumped_version, restored_at)
VALUES (?, ?, ?);

-- name: InRestoreGap :one
-- Whether a cursor lies strictly between the two ends of a restore's gap.
SELECT CAST(EXISTS (
    SELECT 1 FROM restore_gaps
    WHERE restored_version < sqlc.arg(cursor) AND sqlc.arg(cursor) < jumped_version
) AS BOOLEAN) AS in_gap;
