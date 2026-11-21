ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallet_vault_needs_label;
DROP INDEX IF EXISTS idx_wallet_one_main_per_owner;

ALTER TABLE wallets DROP CONSTRAINT IF EXISTS wallet_owner_unique;
ALTER TABLE wallets ADD CONSTRAINT wallet_owner_unique
  UNIQUE (owner_type, owner_id, currency, purpose);

ALTER TABLE wallets DROP COLUMN IF EXISTS target_amount;
