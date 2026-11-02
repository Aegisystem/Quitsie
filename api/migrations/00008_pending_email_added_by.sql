-- The User who gave a pending Member's email, the only one who sees it again
-- (finding 11 of docs/revision-seguridad.md). Everyone else reads the name;
-- the server still compares the email to suggest the Claim.
--
-- Emails stored before this migration have nobody here, so nobody sees them:
-- the activities do not say reliably who typed each one. The previous binary
-- names its columns and ignores this one, so the emails it stores while it
-- runs are hidden the same way, and one it replaces keeps the previous writer.

-- +goose Up

ALTER TABLE members ADD COLUMN email_added_by TEXT REFERENCES users (id);

-- +goose Down

ALTER TABLE members DROP COLUMN email_added_by;
