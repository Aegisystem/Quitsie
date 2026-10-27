-- Initial schema for the data model in docs/arquitectura.md.
--
-- Conventions (kept portable to PostgreSQL, see ADR 0006):
--   * ids are TEXT UUIDv7; expenses and settlements get theirs from the client,
--     so resending an offline batch never duplicates anything (ADR 0001).
--   * amounts are BIGINT in the currency's minor unit (cents); never floats.
--   * currencies are ISO 4217 codes: three upper-case letters.
--   * timestamps are TIMESTAMP in UTC; calendar dates are DATE.
--   * version comes from the global write counter used by sync.

-- +goose Up

CREATE TABLE users (
    id                 TEXT PRIMARY KEY,
    email              TEXT NOT NULL UNIQUE,
    name               TEXT NOT NULL,
    preferred_currency TEXT NOT NULL CHECK (length(preferred_currency) = 3 AND preferred_currency = upper(preferred_currency)),
    google_sub         TEXT UNIQUE,
    created_at         TIMESTAMP NOT NULL,
    deleted_at         TIMESTAMP
);

CREATE TABLE sessions (
    id           TEXT PRIMARY KEY,
    user_id      TEXT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    device       TEXT NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP NOT NULL
);

CREATE INDEX sessions_user_id ON sessions (user_id);

-- A login code is requested by email, possibly before the User exists.
CREATE TABLE login_codes (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    code_hash  TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    attempts   INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX login_codes_email ON login_codes (email);

-- A ledger holds the Expenses and Settlements of a Group or a Friendship (ADR 0008).
CREATE TABLE ledgers (
    id               TEXT PRIMARY KEY,
    kind             TEXT NOT NULL CHECK (kind IN ('group', 'friendship')),
    name             TEXT NOT NULL DEFAULT '',
    currency         TEXT CHECK (length(currency) = 3 AND currency = upper(currency)),
    simplify_debts   BOOLEAN NOT NULL DEFAULT TRUE,
    invite_link_hash TEXT UNIQUE,
    version          BIGINT NOT NULL,
    created_at       TIMESTAMP NOT NULL,
    -- A Group always has a currency; a Friendship never does (each Expense carries its own).
    CHECK ((kind = 'group' AND currency IS NOT NULL) OR (kind = 'friendship' AND currency IS NULL))
);

-- A Member with no user_id is a pending member (or a pending friend in a Friendship).
CREATE TABLE members (
    id         TEXT PRIMARY KEY,
    ledger_id  TEXT NOT NULL REFERENCES ledgers (id),
    user_id    TEXT REFERENCES users (id),
    name       TEXT NOT NULL,
    email      TEXT,
    status     TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'former')),
    created_at TIMESTAMP NOT NULL,
    -- A User holds at most one Member per ledger.
    UNIQUE (ledger_id, user_id)
);

CREATE INDEX members_user_id ON members (user_id);

CREATE TABLE expenses (
    id           TEXT PRIMARY KEY,
    ledger_id    TEXT NOT NULL REFERENCES ledgers (id),
    description  TEXT NOT NULL,
    date         DATE NOT NULL,
    currency     TEXT NOT NULL CHECK (length(currency) = 3 AND currency = upper(currency)),
    total        BIGINT NOT NULL CHECK (total > 0),
    category     TEXT,
    -- Kept only to re-edit the Expense; the OwedShares are stored already computed (ADR 0003).
    split_method TEXT NOT NULL CHECK (split_method IN ('equal', 'exact', 'percentage', 'weights', 'adjustment')),
    created_by   TEXT NOT NULL REFERENCES users (id),
    version      BIGINT NOT NULL,
    created_at   TIMESTAMP NOT NULL,
    deleted_at   TIMESTAMP
);

CREATE INDEX expenses_ledger_version ON expenses (ledger_id, version);

-- PaidShares (Aportes) and OwedShares (Cuotas) each add up to the Expense total.
CREATE TABLE paid_shares (
    expense_id TEXT NOT NULL REFERENCES expenses (id) ON DELETE CASCADE,
    member_id  TEXT NOT NULL REFERENCES members (id),
    amount     BIGINT NOT NULL CHECK (amount > 0),
    PRIMARY KEY (expense_id, member_id)
);

CREATE TABLE owed_shares (
    expense_id TEXT NOT NULL REFERENCES expenses (id) ON DELETE CASCADE,
    member_id  TEXT NOT NULL REFERENCES members (id),
    amount     BIGINT NOT NULL CHECK (amount > 0),
    PRIMARY KEY (expense_id, member_id)
);

CREATE TABLE settlements (
    id             TEXT PRIMARY KEY,
    ledger_id      TEXT NOT NULL REFERENCES ledgers (id),
    from_member_id TEXT NOT NULL REFERENCES members (id),
    to_member_id   TEXT NOT NULL REFERENCES members (id),
    amount         BIGINT NOT NULL CHECK (amount > 0),
    currency       TEXT NOT NULL CHECK (length(currency) = 3 AND currency = upper(currency)),
    date           DATE NOT NULL,
    created_by     TEXT NOT NULL REFERENCES users (id),
    version        BIGINT NOT NULL,
    created_at     TIMESTAMP NOT NULL,
    deleted_at     TIMESTAMP,
    CHECK (from_member_id <> to_member_id)
);

CREATE INDEX settlements_ledger_version ON settlements (ledger_id, version);

-- kind says what changed (for example 'expense_created'); details is a JSON object.
CREATE TABLE activities (
    id         TEXT PRIMARY KEY,
    ledger_id  TEXT NOT NULL REFERENCES ledgers (id),
    user_id    TEXT NOT NULL REFERENCES users (id),
    kind       TEXT NOT NULL,
    details    TEXT NOT NULL DEFAULT '{}',
    created_at TIMESTAMP NOT NULL
);

CREATE INDEX activities_ledger_created ON activities (ledger_id, created_at);

-- +goose Down

DROP TABLE activities;
DROP TABLE settlements;
DROP TABLE owed_shares;
DROP TABLE paid_shares;
DROP TABLE expenses;
DROP TABLE members;
DROP TABLE ledgers;
DROP TABLE login_codes;
DROP TABLE sessions;
DROP TABLE users;
