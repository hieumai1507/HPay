-- Enforce the "ledger is append-only" invariant (docs index.html §8:
-- "Ledger chỉ INSERT — không UPDATE/DELETE — Trigger + quyền DB + review").
--
-- The trigger was documented and even had a CI step named after it, but it was
-- never created: an UPDATE on ledger_entries succeeded silently. Corrections
-- are made by posting reversing entries (reversal_of), never by editing
-- history, so blocking UPDATE and DELETE outright costs nothing legitimate.

CREATE OR REPLACE FUNCTION ledger_entries_append_only()
RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'ledger_entries is append-only: % is not allowed (post a reversing entry instead)', TG_OP;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_ledger_entries_no_update ON ledger_entries;
CREATE TRIGGER trg_ledger_entries_no_update
    BEFORE UPDATE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION ledger_entries_append_only();

DROP TRIGGER IF EXISTS trg_ledger_entries_no_delete ON ledger_entries;
CREATE TRIGGER trg_ledger_entries_no_delete
    BEFORE DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION ledger_entries_append_only();
