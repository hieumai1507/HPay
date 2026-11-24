-- Consolidated Migration 000003: Service Catalog, Transaction Design & Workflow Executions

DROP TYPE IF EXISTS service_status CASCADE;
DROP TYPE IF EXISTS wf_status CASCADE;
DROP TYPE IF EXISTS step_status CASCADE;

CREATE TYPE service_status AS ENUM ('active', 'maintenance', 'disabled');
CREATE TYPE wf_status AS ENUM ('running', 'waiting_approval', 'compensating', 'completed', 'failed', 'compensated', 'recovering');
CREATE TYPE step_status AS ENUM ('completed', 'failed');

CREATE TABLE IF NOT EXISTS services (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            VARCHAR(50) UNIQUE NOT NULL,
    name            VARCHAR(100) NOT NULL,
    full_name       VARCHAR(150),
    service_note    TEXT,
    category        VARCHAR(50) NOT NULL,
    status          service_status NOT NULL DEFAULT 'active',
    transaction_type VARCHAR(50) NOT NULL DEFAULT 'Normal Transaction',
    action_priority VARCHAR(20) NOT NULL DEFAULT 'before_gl',
    confirmation    BOOLEAN NOT NULL DEFAULT true,
    service_target  VARCHAR(100),
    channels        JSONB NOT NULL DEFAULT '["Mobile"]',
    amount_formula  JSONB NOT NULL DEFAULT '{}',
    actions         JSONB NOT NULL DEFAULT '[]',
    input_fields    JSONB NOT NULL DEFAULT '[]',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflow_definitions (
    id        UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name      VARCHAR(60) NOT NULL,
    version   INT NOT NULL,
    steps     JSONB NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT true,
    CONSTRAINT wf_def_unique UNIQUE (name, version)
);

CREATE TABLE IF NOT EXISTS workflow_instances (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    definition_id  UUID NOT NULL REFERENCES workflow_definitions(id),
    transaction_id UUID REFERENCES transactions(id),
    status         wf_status NOT NULL DEFAULT 'running',
    current_step   VARCHAR(40),
    context        JSONB NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_wf_status ON workflow_instances (status);

CREATE TABLE IF NOT EXISTS step_executions (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    instance_id UUID NOT NULL REFERENCES workflow_instances(id),
    step_name   VARCHAR(40) NOT NULL,
    attempt     SMALLINT NOT NULL DEFAULT 1,
    status      step_status NOT NULL,
    input       JSONB,
    output      JSONB,
    error       TEXT,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ
);
-- UNIQUE, not a plain index: (instance, step, attempt) is the E06 idempotency
-- key. The engine also read-checks before acting, but two workers racing the
-- same redelivered message would both pass that check — the constraint is the
-- actual arbiter, and it is what ON CONFLICT targets.
CREATE UNIQUE INDEX IF NOT EXISTS idx_step_exec ON step_executions (instance_id, step_name, attempt);

CREATE TABLE IF NOT EXISTS outbox (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    aggregate    VARCHAR(50) NOT NULL DEFAULT 'system',
    event_type   VARCHAR(50) NOT NULL,
    payload      JSONB NOT NULL,
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_outbox_pending ON outbox (available_at) WHERE published_at IS NULL;

-- Seed Workflow Definitions
INSERT INTO workflow_definitions (id, name, version, steps, is_active)
VALUES
  -- NOTE: the `wallet` handler posts the COMPLETE double entry (debit sender +
  -- credit receiver + fee) in a single step. Splitting it into DEBIT and CREDIT
  -- steps would run that handler twice and move the money twice.
  ('e0000000-0000-0000-0000-000000000001', 'p2p_transfer', 1, '[
    {"name":"VALIDATE","type":"auto","handler":"validator","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"FRAUD_CHECK","type":"auto","handler":"fraud","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":3,"timeout_ms":5000,"on_failure":"compensate","config":{"compensatable":true}},
    {"name":"CONFIRM","type":"auto","handler":"confirmer","retry":3,"timeout_ms":5000,"on_failure":"fail"}
  ]'::jsonb, true),
  ('e0000000-0000-0000-0000-000000000003', 'kyc_verification', 1, '[
    {"name":"SCORE","type":"auto","handler":"kyc_scorer","retry":2,"timeout_ms":30000,"on_failure":"fail"},
    {"name":"CLASSIFY","type":"auto","handler":"kyc_classifier","retry":2,"timeout_ms":5000,"on_failure":"fail"}
  ]'::jsonb, true),
  ('e0000000-0000-0000-0000-000000000002', 'org_disbursement', 1, '[
    {"name":"VALIDATE","type":"auto","handler":"validator","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"FRAUD_CHECK","type":"auto","handler":"fraud","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"WAIT_APPROVAL","type":"human_approval","retry":0,"timeout_ms":86400000,"on_failure":"fail"},
    {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":3,"timeout_ms":5000,"on_failure":"compensate","config":{"compensatable":true}},
    {"name":"CONFIRM","type":"auto","handler":"confirmer","retry":3,"timeout_ms":5000,"on_failure":"fail"}
  ]'::jsonb, true)
ON CONFLICT DO NOTHING;

-- Seed Services
INSERT INTO services (id, code, name, full_name, category, status, action_priority, confirmation, service_target, channels, actions, input_fields)
VALUES
  ('a0000000-0000-0000-0000-000000000001', 'P2P_TRANSFER', 'P2P transfer', 'Subscriber Transfer To Wallet (P2P)', 'transfer', 'active', 'before_gl', true, 'p2p_wallet', '["Mobile", "Web"]', '[
    {"code":"gl_posting","name":"Ledger posting (GL Posting)","type":"GL Posting","entry_point":"model:transdefinition","method":"performAction","required":true},
    {"code":"bank_gateway_notify","name":"Notify partner bank gateway","type":"Directional Integration","entry_point":"https://api.bankpartner.vn/v1/p2p-notify","method":"POST","required":false}
  ]'::jsonb, '[
    {"order": 1, "name": "SERVICEID", "rule": "mapping", "type": "string", "required": true},
    {"order": 2, "name": "AMOUNT", "rule": "mapping", "type": "float", "required": true}
  ]'::jsonb),
  ('a0000000-0000-0000-0000-000000000002', 'MONEY_REQUEST', 'Request money & split bill', 'Request Money & Split Bill Service', 'payment', 'active', 'before_gl', true, 'request_money', '["Mobile"]', '[]'::jsonb, '[]'::jsonb),
  ('a0000000-0000-0000-0000-000000000003', 'TOPUP_BANK', 'Bank top-up', 'Bank Account Cash In Service', 'wallet', 'active', 'before_gl', true, 'cash_in', '["Mobile"]', '[]'::jsonb, '[]'::jsonb),
  ('a0000000-0000-0000-0000-000000000004', 'ORG_DISBURSEMENT', 'Corporate payroll', 'Corporate Payroll Disbursement', 'org', 'active', 'before_gl', true, 'disbursement', '["Web Admin"]', '[]'::jsonb, '[]'::jsonb),
  ('a0000000-0000-0000-0000-000000000005', 'VAULT_SAVINGS', 'Vault savings', 'Vault Savings & Round-up', 'wallet', 'active', 'after_gl', true, 'vault', '["Mobile"]', '[]'::jsonb, '[]'::jsonb)
ON CONFLICT (code) DO NOTHING;
