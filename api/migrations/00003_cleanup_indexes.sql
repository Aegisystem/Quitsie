-- Indexes for the deletes of expired rows that run inside sign-in writes.
-- Without them SQLite scans the whole table while it holds the write lock, and
-- every other write waits: 36,000 live login codes made requests fail with
-- "database is locked" (finding 14 of docs/revision-seguridad.md).

-- +goose Up

CREATE INDEX login_codes_expires_at ON login_codes (expires_at);
CREATE INDEX sessions_last_used_at ON sessions (last_used_at);
CREATE INDEX login_failures_resets_at ON login_failures (resets_at);

-- +goose Down

DROP INDEX login_failures_resets_at;
DROP INDEX sessions_last_used_at;
DROP INDEX login_codes_expires_at;
