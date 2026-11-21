-- Vaults (docs 04 §2.2): a vault is just another wallet with purpose='vault'
-- and a user-chosen label, so it stays inside the ledger and every money
-- invariant applies to it unchanged.

-- Savings goal. Display-only: nothing enforces it, a vault never blocks a
-- withdrawal for being "below target".
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS target_amount BIGINT;

-- The old constraint was (owner_type, owner_id, currency, purpose), which caps
-- a user at ONE vault. Doc 04 calls for label to be part of the key so a user
-- can keep several vaults.
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallet_owner_unique;
ALTER TABLE wallets ADD CONSTRAINT wallet_owner_unique
  UNIQUE (owner_type, owner_id, currency, purpose, label);

-- label is NULL for main wallets and NULL <> NULL in Postgres, so the
-- constraint above no longer prevents a second main wallet. This partial index
-- restores that guarantee for the purposes that must stay singular.
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_one_main_per_owner
  ON wallets (owner_type, owner_id, currency)
  WHERE purpose = 'main';

-- A vault without a label would collide with every other unlabelled vault of
-- the same owner, so require one.
ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallet_vault_needs_label;
ALTER TABLE wallets ADD CONSTRAINT wallet_vault_needs_label
  CHECK (purpose <> 'vault' OR label IS NOT NULL);
