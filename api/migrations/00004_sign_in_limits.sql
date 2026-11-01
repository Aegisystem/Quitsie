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

-- Each login code belongs to the browser that asked for it, through the hash of
-- its flow cookie, so nobody else can spend its attempts (finding 5, ADR 0011).
-- Codes written by an earlier binary have none, and nobody can check them.
ALTER TABLE login_codes ADD COLUMN flow_hash TEXT;

CREATE UNIQUE INDEX login_codes_flow_hash ON login_codes (flow_hash);

-- Browsers that signed in to a User before, by the hash of a token in their
-- known-device cookie (ADR 0011). Their requests skip the limits per email,
-- which anybody can spend, and have limits of their own instead: the wrong
-- guesses of each one count in login_failures, under the hash of its
-- token_hash instead of an email's.
CREATE TABLE known_devices (
    token_hash   TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users (id),
    created_at   TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP NOT NULL
);

CREATE INDEX known_devices_user_id ON known_devices (user_id, last_used_at);

-- +goose Down

DROP TABLE known_devices;
DROP INDEX login_codes_flow_hash;
ALTER TABLE login_codes DROP COLUMN flow_hash;
DROP TABLE mail_sends;
