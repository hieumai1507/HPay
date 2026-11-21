-- Consolidated Migration 000001: Users, Admins, Refresh Tokens, Audit Logs & OTP Challenges

DROP TYPE IF EXISTS kyc_status CASCADE;
DROP TYPE IF EXISTS account_status CASCADE;
DROP TYPE IF EXISTS admin_role CASCADE;

CREATE TYPE account_status AS ENUM ('active', 'locked', 'closed');
CREATE TYPE kyc_status AS ENUM ('none', 'pending', 'manual_review', 'auto_approved', 'verified', 'rejected');
CREATE TYPE admin_role AS ENUM ('viewer', 'kyc_reviewer', 'risk_officer', 'ops', 'superadmin');

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone         VARCHAR(20) UNIQUE NOT NULL,
    email         VARCHAR(160) UNIQUE,
    full_name     VARCHAR(120) NOT NULL,
    password_hash TEXT NOT NULL,
    pin_hash      TEXT,
    kyc_status    kyc_status NOT NULL DEFAULT 'none',
    status        account_status NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id  UUID NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    device_id  VARCHAR(80) NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_refresh_user   ON refresh_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_refresh_family ON refresh_tokens (family_id);

CREATE TABLE IF NOT EXISTS otp_challenges (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash    TEXT NOT NULL,
    purpose      VARCHAR(40) NOT NULL DEFAULT 'transfer',
    attempts     INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 5,
    expires_at   TIMESTAMPTZ NOT NULL,
    -- Named consumed_at to match the auth queries (refresh_tokens keeps its own
    -- separate used_at column — different table, different lifecycle).
    consumed_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         VARCHAR(160) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name     VARCHAR(120) NOT NULL,
    role          admin_role NOT NULL DEFAULT 'viewer',
    status        account_status NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS audit_logs (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id   UUID NOT NULL,
    actor_type VARCHAR(20) NOT NULL,
    action     VARCHAR(60) NOT NULL,
    resource   VARCHAR(120) NOT NULL,
    reason     TEXT,
    before     JSONB,
    after      JSONB,
    ip         VARCHAR(45),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_actor  ON audit_logs (actor_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_action ON audit_logs (action, created_at DESC);

-- Seed dev superadmin
INSERT INTO admin_users (email, password_hash, full_name, role)
VALUES ('admin@HPay.dev',
        '$argon2id$v=19$m=65536,t=3,p=2$nrhMwAWI6chun21JakuPnw$BiPBXH77rUX672GDDGLFhYM4viebYQ1Nw+6MZcYrWrk',
        'Dev Admin', 'superadmin')
ON CONFLICT (email) DO NOTHING;
