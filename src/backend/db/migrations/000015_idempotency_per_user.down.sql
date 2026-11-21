DROP INDEX IF EXISTS uq_money_req_user_idempotency;
ALTER TABLE money_requests DROP COLUMN IF EXISTS request_hash;
ALTER TABLE money_requests DROP COLUMN IF EXISTS idempotency_key;

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS uq_txn_user_idempotency;
ALTER TABLE transactions DROP COLUMN IF EXISTS request_hash;
-- Restores the old global uniqueness. This fails (by design) if two users have
-- used the same key since 000015 was applied: resolve those rows first.
ALTER TABLE transactions ADD CONSTRAINT transactions_idempotency_key_key UNIQUE (idempotency_key);
