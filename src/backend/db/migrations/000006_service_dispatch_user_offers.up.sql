-- Consolidated Migration 000006: Service Dispatch metadata + User Offer assignment
-- Closes audit gaps G1 (dynamic dispatch) and G2/G3 (OfferGuard source of truth).
-- Reuses existing services.actions JSONB for the Action Pipeline (G4) — no new table.

-- 1. Assign an Offer package to each user (source for OfferGuard lookup)
CREATE TABLE IF NOT EXISTS user_offers (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    offer_id    UUID NOT NULL REFERENCES offers(id),
    assigned_by VARCHAR(80) NOT NULL DEFAULT 'system',
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_user_offers_offer ON user_offers (offer_id);

-- 2. Dispatch metadata on services (source for the runtime dispatcher, G1)
ALTER TABLE services ADD COLUMN IF NOT EXISTS workflow_name   VARCHAR(60) NOT NULL DEFAULT 'p2p_transfer';
ALTER TABLE services ADD COLUMN IF NOT EXISTS ledger_txn_type txn_type    NOT NULL DEFAULT 'p2p';
ALTER TABLE services ADD COLUMN IF NOT EXISTS wallet_strategy VARCHAR(30) NOT NULL DEFAULT 'user_to_user';
-- wallet_strategy ∈ {user_to_user, syscash_to_user, org_to_user, user_to_vault}

-- 3. Map each seeded service to its execution profile
UPDATE services SET workflow_name='p2p_transfer',    ledger_txn_type='p2p',             wallet_strategy='user_to_user'    WHERE code='P2P_TRANSFER';
UPDATE services SET workflow_name='p2p_transfer',    ledger_txn_type='p2p',             wallet_strategy='user_to_user'    WHERE code='MONEY_REQUEST';
UPDATE services SET workflow_name='p2p_transfer',    ledger_txn_type='topup',           wallet_strategy='syscash_to_user' WHERE code='TOPUP_BANK';
UPDATE services SET workflow_name='org_disbursement',ledger_txn_type='org_disbursement',wallet_strategy='org_to_user'     WHERE code='ORG_DISBURSEMENT';
UPDATE services SET workflow_name='p2p_transfer',    ledger_txn_type='vault_move',      wallet_strategy='user_to_vault'   WHERE code='VAULT_SAVINGS';

-- 4. Backfill: every existing user gets the customer L2 offer; org actors get Biz Starter.
INSERT INTO user_offers (user_id, offer_id)
  SELECT id, 'b0000000-0000-0000-0000-000000000002' FROM users
ON CONFLICT (user_id) DO NOTHING;

-- Org owner/maker/checkers (seed phones 09000000 11..14) → Business Starter
UPDATE user_offers uo
  SET offer_id = 'b0000000-0000-0000-0000-000000000003'
  FROM users u
  WHERE uo.user_id = u.id AND u.phone IN ('0900000011','0900000012','0900000013','0900000014');

-- 4a. Forward-fix schema/query drift on databases created before 000002/000005
--     were corrected. No-ops on a freshly migrated database.
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();
ALTER TABLE organizations
  ALTER COLUMN code SET DEFAULT ('ORG_' || substr(replace(gen_random_uuid()::text, '-', ''), 1, 12));
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM information_schema.columns
             WHERE table_name = 'otp_challenges' AND column_name = 'used_at') THEN
    ALTER TABLE otp_challenges RENAME COLUMN used_at TO consumed_at;
  END IF;
END $$;
ALTER TABLE approval_requests DROP CONSTRAINT IF EXISTS approval_requests_created_by_fkey;
ALTER TABLE approval_actions DROP CONSTRAINT IF EXISTS approval_actions_checker_id_fkey;

-- Promote the step_executions index to UNIQUE on databases created before
-- 000003 was corrected. Skipped when duplicates already exist so the migration
-- cannot fail on dirty historical data — those need manual reconciliation.
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_indexes WHERE indexname = 'idx_step_exec' AND indexdef ILIKE '%UNIQUE%'
  ) AND NOT EXISTS (
    SELECT 1 FROM step_executions
    GROUP BY instance_id, step_name, attempt HAVING COUNT(*) > 1
  ) THEN
    DROP INDEX IF EXISTS idx_step_exec;
    CREATE UNIQUE INDEX idx_step_exec ON step_executions (instance_id, step_name, attempt);
  END IF;
END $$;

-- 4a-bis. Restore the wallets.balance = SUM(ledger_entries) invariant (docs
--     index §8). System pools were seeded with an un-journalled opening balance
--     because CHECK (balance >= 0) applied to them too, which made the internal
--     recon report drift on every single run. Allow system pools to carry a
--     debit balance (ledger.Post already exempts them) and realign them to
--     their ledger. No-ops on a freshly migrated database.
ALTER TABLE ledger_entries DROP CONSTRAINT IF EXISTS ledger_entries_balance_after_check;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallets_balance_check;
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallet_balance_non_negative;
ALTER TABLE wallets ADD CONSTRAINT wallet_balance_non_negative
  CHECK (balance >= 0 OR owner_type = 'system');

UPDATE wallets w
   SET balance = COALESCE((
         SELECT SUM(CASE le.direction WHEN 'credit' THEN le.amount ELSE -le.amount END)
         FROM ledger_entries le WHERE le.wallet_id = w.id), 0)
 WHERE w.owner_type = 'system'
   AND w.balance <> COALESCE((
         SELECT SUM(CASE le.direction WHEN 'credit' THEN le.amount ELSE -le.amount END)
         FROM ledger_entries le WHERE le.wallet_id = w.id), 0);

-- 4b. Repair legacy system wallet codes on databases created before 000002 was
--     corrected: Go resolves SYSTEM_CASH_IN / SYSTEM_FEE, older seeds wrote
--     SYS_*. Only renames when the correct code is not already present.
UPDATE wallets w SET system_code = 'SYSTEM_CASH_IN'
  WHERE w.system_code = 'SYS_CASH'
    AND NOT EXISTS (SELECT 1 FROM wallets x WHERE x.system_code = 'SYSTEM_CASH_IN');
UPDATE wallets w SET system_code = 'SYSTEM_FEE'
  WHERE w.system_code = 'SYS_FEE'
    AND NOT EXISTS (SELECT 1 FROM wallets x WHERE x.system_code = 'SYSTEM_FEE');
UPDATE wallets w SET system_code = 'SYSTEM_SUSPENSE'
  WHERE w.system_code = 'SYS_SUSPENSE'
    AND NOT EXISTS (SELECT 1 FROM wallets x WHERE x.system_code = 'SYSTEM_SUSPENSE');

-- 5. Make every ACTIVE service available in the customer L1/L2 and Biz Starter offers
--    with sane default limits, so OfferGuard (fail-closed) does not block demo flows.
--    Pre-existing rows (e.g. seeded OFFER_CUST_L2 × P2P) are preserved by ON CONFLICT.
INSERT INTO offer_services (offer_id, service_id, is_enabled, min_amount, max_amount_per_txn, daily_limit, monthly_limit, interval_seconds)
SELECT o.id, s.id, true, 0, 400000, 2000000, 8000000, 0
  FROM offers o
  CROSS JOIN services s
  WHERE o.code IN ('OFFER_CUST_L1','OFFER_CUST_L2','OFFER_BIZ_STARTER')
    AND s.status = 'active'
ON CONFLICT (offer_id, service_id) DO NOTHING;

-- 6. Standard 3-tier auth matrix (pin/otp/2fa) for any offer_service that lacks one.
--    Each tier guarded independently so the pre-seeded L2×P2P rows are untouched.
INSERT INTO offer_auth_matrix (offer_service_id, min_amount, max_amount, auth_method)
SELECT os.id, 0, 8000, 'pin' FROM offer_services os
  WHERE NOT EXISTS (SELECT 1 FROM offer_auth_matrix m WHERE m.offer_service_id = os.id AND m.auth_method = 'pin');
INSERT INTO offer_auth_matrix (offer_service_id, min_amount, max_amount, auth_method)
SELECT os.id, 8000, 200000, 'otp' FROM offer_services os
  WHERE NOT EXISTS (SELECT 1 FROM offer_auth_matrix m WHERE m.offer_service_id = os.id AND m.auth_method = 'otp');
INSERT INTO offer_auth_matrix (offer_service_id, min_amount, max_amount, auth_method)
SELECT os.id, 200000, NULL, '2fa' FROM offer_services os
  WHERE NOT EXISTS (SELECT 1 FROM offer_auth_matrix m WHERE m.offer_service_id = os.id AND m.auth_method = '2fa');
