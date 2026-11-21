DELETE FROM workflow_definitions WHERE name = 'p2p_transfer' AND version = 2;
UPDATE workflow_definitions SET is_active = true WHERE name = 'p2p_transfer' AND version = 1;

DROP INDEX IF EXISTS idx_wallet_round_up_target_unique;
ALTER TABLE wallets DROP COLUMN IF EXISTS round_up_target;
