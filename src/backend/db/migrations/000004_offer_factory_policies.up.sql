-- Consolidated Migration 000004: Offer Factory, Fee Policies & Money Requests

DROP TYPE IF EXISTS offer_role CASCADE;
DROP TYPE IF EXISTS auth_method_enum CASCADE;
DROP TYPE IF EXISTS money_request_status CASCADE;

CREATE TYPE offer_role AS ENUM ('customer', 'agent', 'merchant', 'biller', 'operator');
CREATE TYPE auth_method_enum AS ENUM ('none', 'pin', 'otp', '2fa');
CREATE TYPE money_request_status AS ENUM ('pending', 'paid', 'declined', 'expired', 'canceled');

CREATE TABLE IF NOT EXISTS offers (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code                 VARCHAR(50) UNIQUE NOT NULL,
    name                 VARCHAR(100) NOT NULL,
    role                 offer_role NOT NULL DEFAULT 'customer',
    level                INT NOT NULL DEFAULT 1,
    is_active            BOOLEAN NOT NULL DEFAULT true,
    primary_pocket_max   BIGINT NOT NULL DEFAULT 200000,
    max_bank_accounts    INT NOT NULL DEFAULT 2,
    updated_by           VARCHAR(80) NOT NULL DEFAULT 'system',
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS offer_services (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_id           UUID NOT NULL REFERENCES offers(id) ON DELETE CASCADE,
    service_id         UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    is_enabled         BOOLEAN NOT NULL DEFAULT true,
    min_amount         BIGINT NOT NULL DEFAULT 0,
    max_amount_per_txn BIGINT NOT NULL DEFAULT 400000,
    daily_limit        BIGINT NOT NULL DEFAULT 2000000,
    monthly_limit      BIGINT NOT NULL DEFAULT 8000000,
    interval_seconds   INT NOT NULL DEFAULT 0,
    UNIQUE(offer_id, service_id)
);

CREATE TABLE IF NOT EXISTS offer_auth_matrix (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    offer_service_id UUID NOT NULL REFERENCES offer_services(id) ON DELETE CASCADE,
    min_amount       BIGINT NOT NULL DEFAULT 0,
    max_amount       BIGINT,
    auth_method      auth_method_enum NOT NULL DEFAULT 'pin',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS fee_policies (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    txn_type    txn_type NOT NULL,
    kyc_status  kyc_status,
    fixed_fee   BIGINT NOT NULL DEFAULT 0,
    percent_bp  INT NOT NULL DEFAULT 0,
    min_fee     BIGINT NOT NULL DEFAULT 0,
    max_fee     BIGINT,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    version     INT NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS money_requests (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id      UUID NOT NULL REFERENCES users(id),
    payer_id          UUID NOT NULL REFERENCES users(id),
    amount            BIGINT NOT NULL CHECK (amount > 0),
    note              VARCHAR(255),
    split_group_id    UUID,
    status            money_request_status NOT NULL DEFAULT 'pending',
    paid_txn_id       UUID REFERENCES transactions(id),
    last_reminded_at  TIMESTAMPTZ,
    expires_at        TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT no_self_request CHECK (requester_id <> payer_id)
);
CREATE INDEX IF NOT EXISTS idx_money_req_payer ON money_requests (payer_id, status);

-- Seed default Offers
INSERT INTO offers (id, code, name, role, level, is_active, primary_pocket_max, max_bank_accounts, updated_by)
VALUES
  ('b0000000-0000-0000-0000-000000000001', 'OFFER_CUST_L1', 'Personal (KYC tier 1)', 'customer', 1, true, 200000, 2, 'mungnt'),
  ('b0000000-0000-0000-0000-000000000002', 'OFFER_CUST_L2', 'Personal (KYC tier 2)', 'customer', 2, true, 800000, 5, 'hoangnd'),
  ('b0000000-0000-0000-0000-000000000003', 'OFFER_BIZ_STARTER', 'Business Starter (Maker/Checker L1)', 'merchant', 3, true, 4000000, 10, 'hieumt'),
  ('b0000000-0000-0000-0000-000000000004', 'OFFER_BIZ_ENT', 'Business Enterprise (Maker/Checker L2)', 'merchant', 5, true, 20000000, 20, 'admin.root')
ON CONFLICT (code) DO NOTHING;

-- Seed offer_services & auth matrix
INSERT INTO offer_services (id, offer_id, service_id, is_enabled, min_amount, max_amount_per_txn, daily_limit, monthly_limit, interval_seconds)
VALUES
  ('c0000000-0000-0000-0000-000000000001', 'b0000000-0000-0000-0000-000000000002', 'a0000000-0000-0000-0000-000000000001', true, 0, 400000, 2000000, 8000000, 3)
ON CONFLICT (offer_id, service_id) DO NOTHING;

INSERT INTO offer_auth_matrix (id, offer_service_id, min_amount, max_amount, auth_method)
VALUES
  ('d0000000-0000-0000-0000-000000000001', 'c0000000-0000-0000-0000-000000000001', 0, 8000, 'pin'),
  ('d0000000-0000-0000-0000-000000000002', 'c0000000-0000-0000-0000-000000000001', 8000, 200000, 'otp'),
  ('d0000000-0000-0000-0000-000000000003', 'c0000000-0000-0000-0000-000000000001', 200000, NULL, '2fa')
ON CONFLICT DO NOTHING;

-- Seed default zero fee policies
INSERT INTO fee_policies (txn_type, fixed_fee, percent_bp, is_active)
VALUES
  ('p2p', 25, 0, true),
  ('topup', 0, 0, true),
  ('disbursement', 0, 0, true)
ON CONFLICT DO NOTHING;
