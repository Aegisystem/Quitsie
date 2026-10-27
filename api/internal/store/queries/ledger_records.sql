-- name: ListLedgerExpenses :many
-- Every Expense of a ledger, deleted ones included: they still matter to
-- relabeling the currency and to restoring.
SELECT * FROM expenses
WHERE ledger_id = ?
ORDER BY id;

-- name: ListLedgerPaidShares :many
-- The PaidShares of every Expense in ListLedgerExpenses.
SELECT paid_shares.* FROM paid_shares
JOIN expenses ON expenses.id = paid_shares.expense_id
WHERE expenses.ledger_id = ?
ORDER BY paid_shares.expense_id, paid_shares.member_id;

-- name: ListLedgerOwedShares :many
-- The OwedShares of every Expense in ListLedgerExpenses.
SELECT owed_shares.* FROM owed_shares
JOIN expenses ON expenses.id = owed_shares.expense_id
WHERE expenses.ledger_id = ?
ORDER BY owed_shares.expense_id, owed_shares.member_id;

-- name: ListLedgerSettlements :many
-- Every Settlement of a ledger, deleted ones included.
SELECT * FROM settlements
WHERE ledger_id = ?
ORDER BY id;
