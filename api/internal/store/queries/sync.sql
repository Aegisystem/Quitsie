-- name: PullPageEnd :one
-- The version where a pull of about max_rows rows stops, if it has to. A row
-- counts at its own version or at the one of the User's Member, whichever is
-- later: that is when the User got to see it. So the Changed* queries take every
-- row up to the version this returns, a page never splits a write, and the older
-- rows of a ledger come along with the page where the User joined it.
SELECT CAST(pending.version AS BIGINT) FROM (
    SELECT CASE WHEN ledgers.version > me.version THEN ledgers.version ELSE me.version END AS version
    FROM ledgers
    JOIN members me ON me.ledger_id = ledgers.id
    WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
      AND (ledgers.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
    UNION ALL
    SELECT CASE WHEN members.version > me.version THEN members.version ELSE me.version END
    FROM members
    JOIN members me ON me.ledger_id = members.ledger_id
    WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
      AND (members.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
    UNION ALL
    SELECT CASE WHEN expenses.version > me.version THEN expenses.version ELSE me.version END
    FROM expenses
    JOIN members me ON me.ledger_id = expenses.ledger_id
    WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
      AND (expenses.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
    UNION ALL
    SELECT CASE WHEN expenses.version > me.version THEN expenses.version ELSE me.version END
    FROM paid_shares
    JOIN expenses ON expenses.id = paid_shares.expense_id
    JOIN members me ON me.ledger_id = expenses.ledger_id
    WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
      AND (expenses.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
    UNION ALL
    SELECT CASE WHEN expenses.version > me.version THEN expenses.version ELSE me.version END
    FROM owed_shares
    JOIN expenses ON expenses.id = owed_shares.expense_id
    JOIN members me ON me.ledger_id = expenses.ledger_id
    WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
      AND (expenses.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
    UNION ALL
    SELECT CASE WHEN settlements.version > me.version THEN settlements.version ELSE me.version END
    FROM settlements
    JOIN members me ON me.ledger_id = settlements.ledger_id
    WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
      AND (settlements.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
    UNION ALL
    SELECT CASE WHEN activities.version > me.version THEN activities.version ELSE me.version END
    FROM activities
    JOIN members me ON me.ledger_id = activities.ledger_id
    WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
      AND (activities.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
) AS pending
ORDER BY pending.version
LIMIT 1 OFFSET sqlc.arg(max_rows)::BIGINT;

-- name: ChangedLedgers :many
-- Use ChangesSince. Like every Changed* query, it returns, from the ledgers
-- where the User has an active Member, the rows with a version above the
-- cursor, and every row of a ledger where that Member itself changed above
-- the cursor: that is how the User gets the whole ledger right after joining.
-- It stops at upto, which PullPageEnd gives.
SELECT ledgers.* FROM ledgers
JOIN members me ON me.ledger_id = ledgers.id
WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
  AND (ledgers.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
  AND ledgers.version <= sqlc.arg(upto) AND me.version <= sqlc.arg(upto)
ORDER BY ledgers.version, ledgers.id;

-- name: ChangedMembers :many
-- See ChangedLedgers.
SELECT members.* FROM members
JOIN members me ON me.ledger_id = members.ledger_id
WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
  AND (members.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
  AND members.version <= sqlc.arg(upto) AND me.version <= sqlc.arg(upto)
ORDER BY members.version, members.id;

-- name: ChangedExpenses :many
-- See ChangedLedgers.
SELECT expenses.* FROM expenses
JOIN members me ON me.ledger_id = expenses.ledger_id
WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
  AND (expenses.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
  AND expenses.version <= sqlc.arg(upto) AND me.version <= sqlc.arg(upto)
ORDER BY expenses.version, expenses.id;

-- name: ChangedPaidShares :many
-- The shares of the Expenses in ChangedExpenses; see ChangedLedgers.
SELECT paid_shares.* FROM paid_shares
JOIN expenses ON expenses.id = paid_shares.expense_id
JOIN members me ON me.ledger_id = expenses.ledger_id
WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
  AND (expenses.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
  AND expenses.version <= sqlc.arg(upto) AND me.version <= sqlc.arg(upto)
ORDER BY paid_shares.expense_id, paid_shares.member_id;

-- name: ChangedOwedShares :many
-- The shares of the Expenses in ChangedExpenses; see ChangedLedgers.
SELECT owed_shares.* FROM owed_shares
JOIN expenses ON expenses.id = owed_shares.expense_id
JOIN members me ON me.ledger_id = expenses.ledger_id
WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
  AND (expenses.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
  AND expenses.version <= sqlc.arg(upto) AND me.version <= sqlc.arg(upto)
ORDER BY owed_shares.expense_id, owed_shares.member_id;

-- name: ChangedSettlements :many
-- See ChangedLedgers.
SELECT settlements.* FROM settlements
JOIN members me ON me.ledger_id = settlements.ledger_id
WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
  AND (settlements.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
  AND settlements.version <= sqlc.arg(upto) AND me.version <= sqlc.arg(upto)
ORDER BY settlements.version, settlements.id;

-- name: ChangedActivities :many
-- See ChangedLedgers.
SELECT activities.* FROM activities
JOIN members me ON me.ledger_id = activities.ledger_id
WHERE me.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND me.status = 'active'
  AND (activities.version > sqlc.arg(since) OR me.version > sqlc.arg(since))
  AND activities.version <= sqlc.arg(upto) AND me.version <= sqlc.arg(upto)
ORDER BY activities.version, activities.id;

-- name: DroppedLedgers :many
-- The ledgers where the User's Member became former above the cursor: the
-- User no longer sees them, so their replica must drop them.
SELECT ledger_id FROM members
WHERE user_id = CAST(sqlc.arg(user_id) AS TEXT) AND status = 'former'
  AND version > sqlc.arg(since)
ORDER BY ledger_id;

-- name: RemovedFromLedgers :many
-- The DroppedLedgers where someone removed the User, rather than the User
-- leaving: the write that removes a Member stamps its member_removed Activity
-- with the same version.
SELECT members.ledger_id FROM members
JOIN activities ON activities.ledger_id = members.ledger_id AND activities.version = members.version
WHERE members.user_id = CAST(sqlc.arg(user_id) AS TEXT) AND members.status = 'former'
  AND members.version > sqlc.arg(since) AND activities.kind = 'member_removed'
ORDER BY members.ledger_id;
