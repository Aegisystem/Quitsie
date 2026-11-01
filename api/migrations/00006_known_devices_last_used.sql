-- An index for the delete of stale known devices, which every sign-in runs for
-- every User inside its write. known_devices_user_id leads with user_id, so
-- without this one SQLite scans the whole table with the write lock held
-- (finding 14 of docs/revision-seguridad.md).

-- +goose Up

CREATE INDEX known_devices_last_used_at ON known_devices (last_used_at);

-- +goose Down

DROP INDEX known_devices_last_used_at;
