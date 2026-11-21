-- Postgres cannot drop a single enum value: recreate txn_status without it.
-- Guarded: refuses if any row still uses it, instead of losing data.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM transactions WHERE status = 'awaiting_auth') THEN
    RAISE EXCEPTION 'cannot drop enum value ''awaiting_auth'' from txn_status: % row(s) still reference it',
      (SELECT count(*) FROM transactions WHERE status = 'awaiting_auth');
  END IF;
END $$;

ALTER TYPE txn_status RENAME TO txn_status_old;
CREATE TYPE txn_status AS ENUM ('created', 'pending_approval', 'processing', 'processing_async', 'completed', 'failed', 'reversed');
ALTER TABLE transactions
  ALTER COLUMN status DROP DEFAULT,
  ALTER COLUMN status TYPE txn_status USING status::text::txn_status,
  ALTER COLUMN status SET DEFAULT 'created';
DROP TYPE txn_status_old;