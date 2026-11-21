-- ADR-0017: TOTP second factor for the 2fa band.
-- secret_enc is the AES-256-GCM ciphertext of the shared secret (key from TOTP_ENCRYPTION_KEY),
-- never the secret itself. last_step stops a code being used twice.
CREATE TABLE IF NOT EXISTS user_totp (
    user_id         UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    secret_enc      BYTEA NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT false,
    last_step       BIGINT NOT NULL DEFAULT 0,
    failed_attempts INT NOT NULL DEFAULT 0 CHECK (failed_attempts >= 0),
    locked_until    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    activated_at    TIMESTAMPTZ
);

-- Single-use recovery codes, stored only as SHA-256 hashes.
CREATE TABLE IF NOT EXISTS user_recovery_codes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash  TEXT NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, code_hash)
);
CREATE INDEX IF NOT EXISTS idx_recovery_codes_user ON user_recovery_codes (user_id) WHERE used_at IS NULL;