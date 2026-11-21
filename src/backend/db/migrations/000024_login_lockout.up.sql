-- BE-H8: per-account login lockout for users and admins (BUG-0016).
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS login_failed_attempts INT NOT NULL DEFAULT 0 CHECK (login_failed_attempts >= 0),
  ADD COLUMN IF NOT EXISTS login_locked_until    TIMESTAMPTZ;
ALTER TABLE admin_users
  ADD COLUMN IF NOT EXISTS login_failed_attempts INT NOT NULL DEFAULT 0 CHECK (login_failed_attempts >= 0),
  ADD COLUMN IF NOT EXISTS login_locked_until    TIMESTAMPTZ;