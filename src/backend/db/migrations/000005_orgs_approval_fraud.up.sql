-- Consolidated Migration 000005: Organizations, Maker-Checker Approvals, KYC, Fraud & Recon

DROP TYPE IF EXISTS org_role CASCADE;
DROP TYPE IF EXISTS flag_status CASCADE;
DROP TYPE IF EXISTS recon_result CASCADE;
DROP TYPE IF EXISTS approval_subject CASCADE;
DROP TYPE IF EXISTS approval_status CASCADE;
DROP TYPE IF EXISTS approval_decision CASCADE;

CREATE TYPE org_role AS ENUM ('admin', 'maker', 'checker');
CREATE TYPE flag_status AS ENUM ('open', 'cleared', 'confirmed_fraud');
CREATE TYPE recon_result AS ENUM ('matched', 'amount_mismatch', 'missing_internal', 'missing_external');
CREATE TYPE approval_subject AS ENUM ('org_transaction', 'reversal');
CREATE TYPE approval_status AS ENUM ('pending', 'approved', 'rejected', 'expired');
CREATE TYPE approval_decision AS ENUM ('approve', 'reject');

CREATE TABLE IF NOT EXISTS organizations (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- InsertOrganization only supplies (name, created_by); a generated default
    -- keeps the unique business code satisfied without a schema/query mismatch.
    code          VARCHAR(30) UNIQUE NOT NULL
                  DEFAULT ('ORG_' || substr(replace(gen_random_uuid()::text, '-', ''), 1, 12)),
    name          VARCHAR(120) NOT NULL,
    status        account_status NOT NULL DEFAULT 'active',
    created_by    UUID REFERENCES users(id),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS org_members (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id        UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role          org_role NOT NULL DEFAULT 'maker',
    checker_level SMALLINT CHECK (checker_level IS NULL OR (checker_level >= 1 AND checker_level <= 5)),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT member_unique UNIQUE (org_id, user_id)
);

CREATE TABLE IF NOT EXISTS approval_policies (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id      UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name        VARCHAR(80) NOT NULL,
    min_amount  BIGINT NOT NULL DEFAULT 0,
    max_amount  BIGINT,
    tiers       JSONB NOT NULL DEFAULT '[]',
    version     INT NOT NULL DEFAULT 1,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS approval_policy_tiers (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    policy_id          UUID NOT NULL REFERENCES approval_policies(id) ON DELETE CASCADE,
    level              INT NOT NULL DEFAULT 1,
    tier_level         INT NOT NULL DEFAULT 1,
    min_amount         BIGINT NOT NULL CHECK (min_amount >= 0),
    max_amount         BIGINT CHECK (max_amount IS NULL OR max_amount > min_amount),
    level1_required    INT NOT NULL DEFAULT 0,
    level2_required    INT NOT NULL DEFAULT 0,
    level3_required    INT NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS approval_requests (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    subject_type       approval_subject NOT NULL,
    subject_id         UUID,
    org_id             UUID REFERENCES organizations(id),
    instance_id        UUID REFERENCES workflow_instances(id),
    policy_id          UUID REFERENCES approval_policies(id),
    policy_version     INT NOT NULL DEFAULT 1,
    policy_snapshot    JSONB NOT NULL DEFAULT '{}',
    status             approval_status NOT NULL DEFAULT 'pending',
    current_level      SMALLINT NOT NULL DEFAULT 1,
    reason             TEXT,
    final_decision     VARCHAR(20),
    final_decision_at  TIMESTAMPTZ,
    finalized_at       TIMESTAMPTZ,
    -- No FK: the creator may be an end user (org maker) OR an admin_users row
    -- (reversal / account_lock requests raised from the admin console).
    created_by         UUID,
    expires_at         TIMESTAMPTZ NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS approval_actions (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id  UUID NOT NULL REFERENCES approval_requests(id) ON DELETE CASCADE,
    -- No FK: a checker is an org member (users) for org transactions, but an
    -- admin_users row when ops finalise reversal / account_lock requests.
    checker_id  UUID NOT NULL,
    level       SMALLINT NOT NULL DEFAULT 1,
    action      approval_decision NOT NULL,
    reason      VARCHAR(255),
    acted_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT one_action_per_checker UNIQUE (request_id, checker_id)
);

CREATE TABLE IF NOT EXISTS kyc_requests (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    id_card_url TEXT NOT NULL,
    selfie_url  TEXT NOT NULL,
    ai_score    NUMERIC(5,2),
    ai_verdict  VARCHAR(20),
    ai_reasons  JSONB,
    status      kyc_status NOT NULL DEFAULT 'pending',
    reviewer_id UUID REFERENCES admin_users(id),
    reviewed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS fraud_flags (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    rule_code      VARCHAR(40) NOT NULL,
    score          NUMERIC(5,2) NOT NULL,
    reasons        JSONB NOT NULL DEFAULT '[]',
    status         flag_status NOT NULL DEFAULT 'open',
    reviewer_id    UUID REFERENCES admin_users(id),
    reviewed_at    TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bank_statements (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ref_code  VARCHAR(24) NOT NULL,
    amount    BIGINT NOT NULL,
    stmt_date DATE NOT NULL,
    raw       JSONB NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS reconciliation_runs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_date   DATE NOT NULL UNIQUE,
    total      INT NOT NULL DEFAULT 0,
    matched    INT NOT NULL DEFAULT 0,
    mismatched INT NOT NULL DEFAULT 0,
    ai_summary TEXT,
    report_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS reconciliation_items (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id         UUID NOT NULL REFERENCES reconciliation_runs(id),
    transaction_id UUID REFERENCES transactions(id),
    statement_id   BIGINT REFERENCES bank_statements(id),
    result         recon_result NOT NULL,
    detail         JSONB
);

CREATE TABLE IF NOT EXISTS ai_keys (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider       VARCHAR(40) NOT NULL,
    name           VARCHAR(100) NOT NULL,
    key_ciphertext TEXT,
    api_key        TEXT,
    key_masked     VARCHAR(30) NOT NULL DEFAULT 'sk-****',
    is_active      BOOLEAN NOT NULL DEFAULT true,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ai_function_settings (
    function_name VARCHAR(60) PRIMARY KEY,
    provider      VARCHAR(40) NOT NULL,
    model         VARCHAR(80),
    model_name    VARCHAR(80) NOT NULL DEFAULT 'claude-3-5-sonnet-20241022',
    key_id        UUID REFERENCES ai_keys(id),
    api_key       VARCHAR(255),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed default AI settings
INSERT INTO ai_function_settings (function_name, provider, model_name, key_id)
VALUES
  -- Default to the offline mock: no API key ships with the repo, and an
  -- 'anthropic' default with a NULL key silently disables advisories.
  -- Ops switch the provider to anthropic/openai from the admin AI settings.
  ('kyc_scoring', 'mock', 'mock', NULL),
  ('fraud_advising', 'mock', 'mock', NULL)
ON CONFLICT (function_name) DO NOTHING;
