-- Who was removed from a Group, as opposed to who left: they cannot come back
-- through the Invite link until a linked Member readmits them (finding 8 of
-- docs/revision-seguridad.md).
--
-- The previous binary names its columns, so it ignores this one and leaves it
-- FALSE on the Members it creates. While it runs, a removed User can come back
-- as before, and the flag stays on their active Member: it only counts on a
-- former one, and leaving or being removed again under this binary rewrites it.

-- +goose Up

ALTER TABLE members ADD COLUMN removed BOOLEAN NOT NULL DEFAULT FALSE;

-- +goose Down

ALTER TABLE members DROP COLUMN removed;
