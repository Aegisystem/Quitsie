-- Wrong guesses at login codes per email, across codes, so that a new code buys
-- no more guesses and a restart forgets none.

-- +goose Up

-- The hash and not the email: this row outlives the code, and can outlive the
-- account, and the rule is that a deleted account leaves no email behind.
CREATE TABLE login_failures (
    email_hash TEXT PRIMARY KEY,
    failures   INTEGER NOT NULL,
    resets_at  TIMESTAMP NOT NULL
);

-- +goose Down

DROP TABLE login_failures;
