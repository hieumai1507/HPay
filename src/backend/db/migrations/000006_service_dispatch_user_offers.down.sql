-- Down 000006: reverse service dispatch metadata + user offer assignment.
-- offer_services / offer_auth_matrix rows created here are left in place on down
-- (they are additive demo data guarded by ON CONFLICT); only structures added by
-- this migration are removed so up→down→up (E18) round-trips cleanly.

ALTER TABLE services DROP COLUMN IF EXISTS wallet_strategy;
ALTER TABLE services DROP COLUMN IF EXISTS ledger_txn_type;
ALTER TABLE services DROP COLUMN IF EXISTS workflow_name;

DROP INDEX IF EXISTS idx_user_offers_offer;
DROP TABLE IF EXISTS user_offers;
