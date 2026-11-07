-- Every write to a ledger tells the server which ledger changed and at which
-- version of sync, so it can let open devices and push notifications know
-- (ADR 0013). PostgreSQL delivers a notification at commit, and only once for
-- equal payloads of one transaction, so a write of many rows sends one.

-- +goose Up
-- +goose StatementBegin
CREATE FUNCTION notify_ledger_change() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    changed jsonb := to_jsonb(NEW);
BEGIN
    -- A ledger row has its own id; every other row points to its ledger.
    PERFORM pg_notify('quitsie_changes', json_build_object(
        'ledgerId', COALESCE(changed->>'ledger_id', changed->>'id'),
        'version', (changed->>'version')::bigint
    )::text);
    RETURN NULL;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER ledgers_notify AFTER INSERT OR UPDATE ON ledgers
    FOR EACH ROW EXECUTE FUNCTION notify_ledger_change();
CREATE TRIGGER members_notify AFTER INSERT OR UPDATE ON members
    FOR EACH ROW EXECUTE FUNCTION notify_ledger_change();
CREATE TRIGGER expenses_notify AFTER INSERT OR UPDATE ON expenses
    FOR EACH ROW EXECUTE FUNCTION notify_ledger_change();
CREATE TRIGGER settlements_notify AFTER INSERT OR UPDATE ON settlements
    FOR EACH ROW EXECUTE FUNCTION notify_ledger_change();
CREATE TRIGGER activities_notify AFTER INSERT OR UPDATE ON activities
    FOR EACH ROW EXECUTE FUNCTION notify_ledger_change();

-- +goose Down
DROP TRIGGER activities_notify ON activities;
DROP TRIGGER settlements_notify ON settlements;
DROP TRIGGER expenses_notify ON expenses;
DROP TRIGGER members_notify ON members;
DROP TRIGGER ledgers_notify ON ledgers;
DROP FUNCTION notify_ledger_change();
