-- name: CreateActivity :exec
-- kind says what changed (for example 'expense_created'); details is a JSON object.
INSERT INTO activities (id, ledger_id, user_id, kind, details, version, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListActivities :many
-- Newest first. Versions follow commit order, so they order Activities even
-- when two share the same created_at.
SELECT * FROM activities
WHERE ledger_id = ?
ORDER BY version DESC, created_at DESC, id DESC;
