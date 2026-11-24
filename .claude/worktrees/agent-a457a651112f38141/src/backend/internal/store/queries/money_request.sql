-- name: InsertMoneyRequest :one
-- split_group_id: pass uuid.Nil for a standalone request (NULLIF -> NULL).
-- created_at uses clock_timestamp() so rows of one split keep their insertion order (replay order).
INSERT INTO money_requests (requester_id, payer_id, amount, note, split_group_id, expires_at, idempotency_key, request_hash, created_at)
VALUES ($1, $2, $3, $4, NULLIF(sqlc.arg(split_group_id)::uuid, '00000000-0000-0000-0000-000000000000'::uuid), $5, $6, $7, clock_timestamp())
RETURNING id, requester_id, payer_id, amount, note, status, expires_at, created_at;

-- name: ListMoneyRequestsByIdempotencyKey :many
-- Per requester (ADR-0011). A split returns one row per payer, oldest first.
SELECT id, amount, status, expires_at, request_hash,
       COALESCE(split_group_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS split_group_id
FROM money_requests WHERE requester_id = $1 AND idempotency_key = $2
ORDER BY created_at, id;

-- name: GetMoneyRequestForUpdate :one
-- E19: the row lock is what actually closes the expiry race, not a status flag.
SELECT id, requester_id, payer_id, amount, note,
       COALESCE(split_group_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS split_group_id,
       status, expires_at
FROM money_requests WHERE id = $1 FOR UPDATE;

-- name: GetMoneyRequest :one
SELECT id, requester_id, payer_id, amount, note,
       COALESCE(split_group_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS split_group_id,
       status,
       COALESCE(paid_txn_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS paid_txn_id,
       expires_at, created_at, last_reminded_at
FROM money_requests WHERE id = $1;

-- name: FinalizeMoneyRequestPaid :execrows
-- Guarded: only pending, unexpired requests may become paid (E19).
UPDATE money_requests SET status = 'paid', paid_txn_id = $2
WHERE id = $1 AND status = 'pending' AND expires_at > now();

-- name: DeclineMoneyRequest :execrows
UPDATE money_requests SET status = 'declined'
WHERE id = $1 AND payer_id = $2 AND status = 'pending';

-- name: CancelMoneyRequest :execrows
UPDATE money_requests SET status = 'canceled'
WHERE id = $1 AND requester_id = $2 AND status = 'pending';

-- name: ListInboxPending :many
-- "My Tasks" for a payer: requests still pending AND not past their deadline
-- (mirrors the E19 check so the inbox never shows something Pay() would reject).
SELECT r.id, r.requester_id, u.full_name AS requester_name, r.amount, r.note,
       COALESCE(r.split_group_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS split_group_id,
       r.expires_at, r.created_at, r.last_reminded_at
FROM money_requests r JOIN users u ON u.id = r.requester_id
WHERE r.payer_id = $1 AND r.status = 'pending' AND r.expires_at > now()
ORDER BY r.created_at DESC;

-- name: ListSentByRequester :many
SELECT r.id, r.payer_id, u.full_name AS payer_name, r.amount, r.note,
       COALESCE(r.split_group_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS split_group_id,
       r.status, r.expires_at, r.created_at, r.last_reminded_at
FROM money_requests r JOIN users u ON u.id = r.payer_id
WHERE r.requester_id = $1
ORDER BY r.created_at DESC
LIMIT 50;

-- name: ListBySplitGroup :many
SELECT r.id, r.payer_id, u.full_name AS payer_name, r.amount, r.status,
       COALESCE(r.paid_txn_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS paid_txn_id
FROM money_requests r JOIN users u ON u.id = r.payer_id
WHERE r.split_group_id = sqlc.arg(split_group_id)::uuid
ORDER BY r.created_at;

-- name: ListDueMoneyRequestIDs :many
SELECT id FROM money_requests
WHERE status = 'pending' AND expires_at < now()
ORDER BY expires_at
LIMIT $1;

-- name: ExpireMoneyRequest :execrows
-- Guarded transition, same claim shape as approval.FinalizeApprovalRequest (E10 pattern).
UPDATE money_requests SET status = 'expired'
WHERE id = $1 AND status = 'pending';

-- name: RemindMoneyRequestGuarded :one
UPDATE money_requests
SET last_reminded_at = NOW()
WHERE id = $1
  AND requester_id = $2
  AND status = 'pending'
  AND (last_reminded_at IS NULL OR last_reminded_at < NOW() - INTERVAL '24 hours')
RETURNING id, last_reminded_at;
