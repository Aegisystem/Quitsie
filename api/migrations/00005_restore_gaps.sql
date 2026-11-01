-- The versions each restore of a backup skipped, so that GET /sync can tell a
-- cursor that may point at lost rows (docs/arquitectura.md, "Restaurar un
-- respaldo").

-- +goose Up

-- One row per restore. restored_version is version_counter as the backup had
-- it, and jumped_version is where the server moved it on its first start after
-- the restore. The database that was lost may have handed out the versions in
-- between, and a device whose cursor is among them may hold rows that no
-- longer exist, so GET /sync refuses those cursors. Both ends are cursors this
-- database hands out.
CREATE TABLE restore_gaps (
    restored_version BIGINT PRIMARY KEY,
    jumped_version   BIGINT NOT NULL,
    restored_at      TIMESTAMP NOT NULL,
    CHECK (jumped_version > restored_version)
);

-- +goose Down

DROP TABLE restore_gaps;
