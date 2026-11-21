-- Consolidated Migration 000002: Wallets, Double-Entry Ledger & System Accounts

DROP TYPE IF EXISTS wallet_status CASCADE;
DROP TYPE IF EXISTS txn_type CASCADE;
DROP TYPE IF EXISTS txn_status CASCADE;
DROP TYPE IF EXISTS entry_direction CASCADE;
DROP TYPE IF EXISTS owner_type CASCADE;

CREATE TYPE wallet_status AS ENUM ('active', 'locked', 'closed');
CREATE TYPE txn_type AS ENUM ('topup', 'p2p', 'disbursement', 'org_disbursement', 'reversal', 'vault_move');
CREATE TYPE txn_status AS ENUM ('created', 'pending_approval', 'processing', 'processing_async', 'completed', 'failed', 'reversed');
CREATE TYPE entry_direction AS ENUM ('debit', 'credit');
CREATE TYPE owner_type AS ENUM ('user', 'organization', 'system');

CREATE TABLE IF NOT EXISTS wallets (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_type  owner_type NOT NULL,
    owner_id    UUID NOT NULL,
    currency    VARCHAR(3) NOT NULL DEFAULT 'VND',
    purpose     VARCHAR(20) NOT NULL DEFAULT 'main',
    label       VARCHAR(60),
    balance     BIGINT NOT NULL DEFAULT 0,
    status      wallet_status NOT NULL DEFAULT 'active',
    version     INT NOT NULL DEFAULT 1,
    system_code VARCHAR(40),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- Written by ApplyWalletDelta on every balance change (optimistic locking trail).
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT wallet_owner_unique UNIQUE (owner_type, owner_id, currency, purpose),
    -- E02's last line of defence for customer money. System pools (cash-in,
    -- fee, suspense) are funding/contra accounts that legitimately carry a
    -- debit balance — ledger.Post already exempts them, and forcing them
    -- non-negative is what previously required a fake opening balance that
    -- broke the wallets.balance = SUM(ledger_entries) invariant.
    CONSTRAINT wallet_balance_non_negative CHECK (balance >= 0 OR owner_type = 'system')
);

CREATE TABLE IF NOT EXISTS ledger_accounts (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code       VARCHAR(40) UNIQUE NOT NULL,
    name       VARCHAR(100) NOT NULL,
    type       VARCHAR(20) NOT NULL,
    wallet_id  UUID REFERENCES wallets(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ref_code           VARCHAR(24) UNIQUE NOT NULL,
    type               txn_type NOT NULL,
    sender_wallet_id   UUID REFERENCES wallets(id),
    receiver_wallet_id UUID REFERENCES wallets(id),
    amount             BIGINT NOT NULL CHECK (amount > 0),
    fee                BIGINT NOT NULL DEFAULT 0 CHECK (fee >= 0),
    status             txn_status NOT NULL DEFAULT 'created',
    initiated_by       UUID NOT NULL REFERENCES users(id),
    idempotency_key    VARCHAR(80) UNIQUE,
    failure_reason     TEXT,
    note               VARCHAR(255),
    reversal_of        UUID REFERENCES transactions(id),
    completed_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_txn_sender   ON transactions (sender_wallet_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_txn_receiver ON transactions (receiver_wallet_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_txn_ref      ON transactions (ref_code);

CREATE TABLE IF NOT EXISTS ledger_entries (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    transaction_id UUID NOT NULL REFERENCES transactions(id),
    wallet_id      UUID REFERENCES wallets(id),
    account_id     UUID REFERENCES ledger_accounts(id),
    direction      entry_direction NOT NULL,
    amount         BIGINT NOT NULL CHECK (amount > 0),
    -- Running total for audit, not a guard: it mirrors wallets.balance, which
    -- carries the non-negative constraint for customer wallets. System pools
    -- run negative by design, so this column must allow it.
    balance_after  BIGINT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_ledger_account ON ledger_entries (account_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ledger_wallet  ON ledger_entries (wallet_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_ledger_txn     ON ledger_entries (transaction_id);

-- Seed System Wallets & Accounts
INSERT INTO users (id, phone, full_name, password_hash)
VALUES ('00000000-0000-0000-0000-000000000000', '0000000000', 'SYSTEM', 'disabled')
ON CONFLICT DO NOTHING;

INSERT INTO wallets (id, owner_type, owner_id, currency, purpose, balance, system_code)
VALUES
  -- system_code MUST match the lookups in Go (wallet/org/reversal/transfer and
  -- cmd/seed all resolve SYSTEM_CASH_IN / SYSTEM_FEE). ledger_accounts.code
  -- below keeps its own SYS_* naming — different column, different concept.
  -- Opening balance 0 on purpose: every movement is journalled, so the pool's
  -- balance is exactly SUM(ledger_entries). Cash-in goes negative as money
  -- flows out to user wallets, which is the correct sign for a funding pool.
  ('00000000-0000-0000-0000-000000000001', 'system', '00000000-0000-0000-0000-000000000000', 'VND', 'system_cash', 0, 'SYSTEM_CASH_IN'),
  ('00000000-0000-0000-0000-000000000002', 'system', '00000000-0000-0000-0000-000000000000', 'VND', 'system_fee', 0, 'SYSTEM_FEE'),
  ('00000000-0000-0000-0000-000000000003', 'system', '00000000-0000-0000-0000-000000000000', 'VND', 'system_suspense', 0, 'SYSTEM_SUSPENSE')
ON CONFLICT DO NOTHING;

INSERT INTO ledger_accounts (id, code, name, type, wallet_id)
VALUES
  ('00000000-0000-0000-0000-000000000001', 'SYS_CASH', 'System Cash Pool', 'asset', '00000000-0000-0000-0000-000000000001'),
  ('00000000-0000-0000-0000-000000000002', 'SYS_FEE', 'System Fee Pool', 'income', '00000000-0000-0000-0000-000000000002'),
  ('00000000-0000-0000-0000-000000000003', 'SYS_SUSPENSE', 'System Suspense Pool', 'liability', '00000000-0000-0000-0000-000000000003')
ON CONFLICT DO NOTHING;
