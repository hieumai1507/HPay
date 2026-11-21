-- Postgres can't DROP a single enum value directly, so this recreates
-- approval_status without 'cancelled' and swaps every column over. Safe as
-- long as no row is currently 'cancelled' (true on a fresh/CI database,
-- which is what the E18 up->down->up gate exercises) — guarded so a real
-- database with cancelled requests fails loudly instead of losing data.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM approval_requests WHERE status = 'cancelled') THEN
    RAISE EXCEPTION 'cannot drop enum value ''cancelled'' from approval_status: % row(s) still reference it — resolve or archive them first',
      (SELECT count(*) FROM approval_requests WHERE status = 'cancelled');
  END IF;
END $$;

ALTER TYPE approval_status RENAME TO approval_status_old;
CREATE TYPE approval_status AS ENUM ('pending', 'approved', 'rejected', 'expired');
ALTER TABLE approval_requests
  ALTER COLUMN status DROP DEFAULT,
  ALTER COLUMN status TYPE approval_status USING status::text::approval_status,
  ALTER COLUMN status SET DEFAULT 'pending';
DROP TYPE approval_status_old;
