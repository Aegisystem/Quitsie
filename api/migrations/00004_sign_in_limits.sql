-- Limits on signing in that a restart, which every deploy does, must not
-- forget.

-- +goose Up

-- Login-code emails sent per clock hour, so the server stops sending before it
-- spends the free quota of its email provider (finding 6 of
-- docs/revision-seguridad.md). Rows older than a day are deleted.
CREATE TABLE mail_sends (
    hour TIMESTAMP PRIMARY KEY,
    sent INTEGER NOT NULL
);

-- +goose Down

DROP TABLE mail_sends;
