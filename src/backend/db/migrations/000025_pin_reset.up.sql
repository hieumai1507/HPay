-- ADR-0016: forgotten PIN reset. pin_reset_at starts the cooling period after a reset.
ALTER TABLE users ADD COLUMN IF NOT EXISTS pin_reset_at TIMESTAMPTZ;