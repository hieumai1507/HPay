DROP TRIGGER IF EXISTS trg_ledger_entries_no_delete ON ledger_entries;
DROP TRIGGER IF EXISTS trg_ledger_entries_no_update ON ledger_entries;
DROP FUNCTION IF EXISTS ledger_entries_append_only();
