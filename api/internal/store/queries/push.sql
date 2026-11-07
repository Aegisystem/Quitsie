-- name: DeleteOtherPushSubscriptionOfSession :exec
-- A session holds one subscription: a browser that subscribes again with
-- another endpoint replaces the old one.
DELETE FROM push_subscriptions
WHERE session_id = $1 AND endpoint <> $2;

-- name: SavePushSubscription :exec
-- An endpoint moves to the session that saves it last, so a browser where
-- another User signs in stops getting the notifications of the first.
INSERT INTO push_subscriptions (endpoint, session_id, p256dh, auth, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (endpoint) DO UPDATE
SET session_id = excluded.session_id,
    p256dh     = excluded.p256dh,
    auth       = excluded.auth,
    created_at = excluded.created_at;

-- name: DeletePushSubscriptionOfSession :exec
DELETE FROM push_subscriptions
WHERE session_id = $1;

-- name: DeletePushSubscription :exec
-- The push service said the endpoint no longer exists.
DELETE FROM push_subscriptions
WHERE endpoint = $1;

-- name: ListPushSubscriptionsOfUser :many
-- The devices of a User with notifications on, in sessions that have not
-- expired.
SELECT push_subscriptions.*
FROM push_subscriptions
JOIN sessions ON sessions.id = push_subscriptions.session_id
WHERE sessions.user_id = sqlc.arg(user_id) AND sessions.last_used_at > sqlc.arg(used_after)
ORDER BY push_subscriptions.created_at, push_subscriptions.endpoint;

-- name: UserHasPushSubscription :one
SELECT EXISTS (
    SELECT 1
    FROM push_subscriptions
    JOIN sessions ON sessions.id = push_subscriptions.session_id
    WHERE sessions.user_id = sqlc.arg(user_id) AND sessions.last_used_at > sqlc.arg(used_after)
);

-- name: TakePushProgress :one
-- Holds the cursor until the transaction ends, so two instances of the
-- server never go through the same Activities.
SELECT version FROM push_progress
WHERE id = 1
FOR UPDATE;

-- name: SetPushProgress :exec
UPDATE push_progress
SET version = $1
WHERE id = 1;

-- name: ListNotifiableActivities :many
-- New Expenses and Settlements after the cursor, oldest first. Those created
-- before fresh_after are left out: a notification about them would be late.
SELECT * FROM activities
WHERE version > sqlc.arg(after) AND version <= sqlc.arg(upto)
  AND kind IN ('expense_created', 'settlement_created')
  AND created_at > sqlc.arg(fresh_after)
ORDER BY version, id;
