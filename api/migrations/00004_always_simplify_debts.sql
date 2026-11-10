-- Debt simplification is always on: the app no longer offers turning it off
-- (2026-11-10). A Group that had it off turns it on with a version of its own,
-- like any write, so the next sync sends its simplified Debts to every device
-- and the trigger tells the open ones. Deleted ledgers stay as they were, and
-- with no Group to change the version does not move.

-- +goose Up
WITH next AS (
    UPDATE version_counter
    SET value = value + 1
    WHERE id = 1 AND EXISTS (SELECT 1 FROM ledgers WHERE NOT simplify_debts AND deleted_at IS NULL)
    RETURNING value
)
UPDATE ledgers
SET simplify_debts = TRUE,
    version = (SELECT value FROM next)
WHERE NOT simplify_debts AND deleted_at IS NULL;

-- +goose Down
-- Which Groups had it off is not kept, so there is nothing to undo.
SELECT 1;
