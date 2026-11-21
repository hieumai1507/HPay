-- ADR-0008 / ADR-0014: server-side transaction PIN with an attempt counter and lockout.
-- users.pin_hash already exists (migration 000001) and was never used.
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS pin_failed_attempts INT NOT NULL DEFAULT 0 CHECK (pin_failed_attempts >= 0),
  ADD COLUMN IF NOT EXISTS pin_locked_until    TIMESTAMPTZ;