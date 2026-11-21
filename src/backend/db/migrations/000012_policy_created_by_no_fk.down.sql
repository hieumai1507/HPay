-- Safe to restore only if every existing created_by still points at a real
-- users row (an admin-created policy row would violate this) — guard like
-- migration 000011's down.
DO $$
BEGIN
  IF EXISTS (
    SELECT 1 FROM approval_policies p
    WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = p.created_by)
  ) THEN
    RAISE EXCEPTION 'cannot restore approval_policies_created_by_fkey: at least one policy was created by an admin_users id, not a users id';
  END IF;
END $$;

ALTER TABLE approval_policies
  ADD CONSTRAINT approval_policies_created_by_fkey FOREIGN KEY (created_by) REFERENCES users(id);
