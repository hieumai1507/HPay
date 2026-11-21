-- ADR-0011 / audit BE-H1: idempotency keys are scoped per user and bound to
-- the request that first used them.
--
-- transactions.initiated_by is NOT NULL (000002), so (initiated_by,
-- idempotency_key) is always fully defined; rows with a NULL key stay
-- unconstrained (NULLs are distinct in a UNIQUE constraint).
--
-- request_hash is NULL for every pre-existing row: NULL means "unknown
-- payload, accept replay" (the application treats it as a match).

ALTER TABLE transactions ADD COLUMN IF NOT EXISTS request_hash VARCHAR(64);

ALTER TABLE transactions DROP CONSTRAINT IF EXISTS transactions_idempotency_key_key;
ALTER TABLE transactions
    ADD CONSTRAINT uq_txn_user_idempotency UNIQUE (initiated_by, idempotency_key);

-- money_requests had no idempotency at all (BE-H1 note): creating a request or
-- a split twice produced duplicates. Same scoping: per requester.
ALTER TABLE money_requests ADD COLUMN IF NOT EXISTS idempotency_key VARCHAR(80);
ALTER TABLE money_requests ADD COLUMN IF NOT EXISTS request_hash VARCHAR(64);

-- Backstop for single requests. A split writes one row per payer under the same
-- key, so splits are serialized by an advisory lock in the service instead.
CREATE UNIQUE INDEX IF NOT EXISTS uq_money_req_user_idempotency
    ON money_requests (requester_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL AND split_group_id IS NULL;
