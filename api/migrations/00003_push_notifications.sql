-- Push notifications of new Expenses and Settlements (ADR 0013).

-- +goose Up

-- A browser that turned notifications on, one per session: signing out,
-- closing the session on another device or its expiry deletes it, and so does
-- deleting the account. endpoint is the URL of the browser's push service
-- (Apple, Google, Mozilla or Microsoft) for that browser; p256dh and auth are
-- the keys that encrypt each notification for it alone (RFC 8291). An
-- endpoint belongs to one session: the last User to sign in on the browser.
CREATE TABLE push_subscriptions (
    endpoint   TEXT PRIMARY KEY,
    session_id TEXT NOT NULL UNIQUE REFERENCES sessions (id) ON DELETE CASCADE,
    p256dh     TEXT NOT NULL,
    auth       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

-- The version up to which the notifier went through the Activities. Each
-- instance of the server takes it with FOR UPDATE, so each Activity is
-- notified once. It starts at the current version: older Activities are
-- never notified.
CREATE TABLE push_progress (
    id      BIGINT PRIMARY KEY CHECK (id = 1),
    version BIGINT NOT NULL
);

INSERT INTO push_progress (id, version) SELECT 1, value FROM version_counter;

-- The notifier reads the Activities of every ledger after its cursor.
CREATE INDEX activities_version ON activities (version);

-- +goose Down

DROP INDEX activities_version;
DROP TABLE push_progress;
DROP TABLE push_subscriptions;
