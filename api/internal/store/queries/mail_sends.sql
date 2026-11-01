-- name: ListMailSends :many
SELECT * FROM mail_sends
WHERE hour > ?
ORDER BY hour;

-- name: CountMailSend :exec
INSERT INTO mail_sends (hour, sent)
VALUES (?, 1)
ON CONFLICT (hour) DO UPDATE
SET sent = mail_sends.sent + 1;

-- name: UncountMailSend :exec
UPDATE mail_sends
SET sent = sent - 1
WHERE hour = ? AND sent > 0;

-- name: DeleteOldMailSends :exec
DELETE FROM mail_sends
WHERE hour <= ?;
