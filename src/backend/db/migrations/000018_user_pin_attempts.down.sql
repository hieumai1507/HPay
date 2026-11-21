ALTER TABLE users
  DROP COLUMN IF EXISTS pin_locked_until,
  DROP COLUMN IF EXISTS pin_failed_attempts;