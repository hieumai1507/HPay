-- Round-up vault step (docs 04 §2.2, stretch): every P2P spend transaction can
-- optionally sweep its rounding difference into one designated vault. Round-up
-- is best-effort and opt-in — a wallet with round_up_target=false (default)
-- never participates.
ALTER TABLE wallets ADD COLUMN IF NOT EXISTS round_up_target BOOLEAN NOT NULL DEFAULT false;

-- At most one round-up target vault per owner at a time. Scoped to
-- purpose='vault' so a main wallet can never accidentally become a target.
CREATE UNIQUE INDEX IF NOT EXISTS idx_wallet_round_up_target_unique
  ON wallets (owner_type, owner_id, currency)
  WHERE round_up_target AND purpose = 'vault';

-- New workflow definition version: v1 keeps running for any in-flight
-- instance still bound to it; new transactions start on v2 which appends the
-- optional ROUND_UP step after CONFIRM.
UPDATE workflow_definitions SET is_active = false WHERE name = 'p2p_transfer' AND version = 1;

INSERT INTO workflow_definitions (name, version, steps, is_active)
VALUES (
  'p2p_transfer', 2, '[
    {"name":"VALIDATE","type":"auto","handler":"validator","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"FRAUD_CHECK","type":"auto","handler":"fraud","retry":2,"timeout_ms":10000,"on_failure":"fail"},
    {"name":"MOVE_FUNDS","type":"auto","handler":"wallet","retry":3,"timeout_ms":5000,"on_failure":"compensate","config":{"compensatable":true}},
    {"name":"CONFIRM","type":"auto","handler":"confirmer","retry":3,"timeout_ms":5000,"on_failure":"fail"},
    {"name":"ROUND_UP","type":"auto","handler":"round_up","retry":3,"timeout_ms":5000,"on_failure":"fail"}
  ]'::jsonb, true
)
ON CONFLICT ON CONSTRAINT wf_def_unique DO NOTHING;
