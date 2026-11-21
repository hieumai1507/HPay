-- Postgres cannot DROP a single enum value, so this recreates approval_status
-- without 'needs_manual' and swaps the column over. Guarded: if any request is
-- still 'needs_manual' it fails loudly instead of losing the operator's work.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM approval_requests WHERE status = 'needs_manual') THEN
    RAISE EXCEPTION 'cannot drop enum value ''needs_manual'' from approval_status: % row(s) still reference it - resolve them first',
      (SELECT count(*) FROM approval_requests WHERE status = 'needs_manual');
  END IF;
END $$;

ALTER TYPE approval_status RENAME TO approval_status_old;
CREATE TYPE approval_status AS ENUM ('pending', 'approved', 'rejected', 'expired', 'cancelled');
ALTER TABLE approval_requests
  ALTER COLUMN status DROP DEFAULT,
  ALTER COLUMN status TYPE approval_status USING status::text::approval_status,
  ALTER COLUMN status SET DEFAULT 'pending';
DROP TYPE approval_status_old;