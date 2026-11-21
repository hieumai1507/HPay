-- name: InsertTransaction :one
INSERT INTO transactions (ref_code, idempotency_key, type, status, sender_wallet_id, receiver_wallet_id, amount, fee, note, initiated_by)
VALUES ($1, $2, $3, 'processing', $4, $5, $6, $7, $8, $9)
RETURNING id, ref_code, status, created_at;

-- name: GetTransactionByIdempotencyKey :one
SELECT id, ref_code, type, status, sender_wallet_id, receiver_wallet_id, amount, fee, note, created_at, completed_at
FROM transactions WHERE idempotency_key = $1;

-- name: GetTransactionByRefCode :one
-- Counterparty identity joined for the owner-facing detail view (m3-api-support).
-- Only user-owned wallets resolve to a person; system/org wallets yield NULLs.
SELECT t.id, t.ref_code, t.type, t.status, t.sender_wallet_id, t.receiver_wallet_id,
       t.amount, t.fee, t.note, t.created_at, t.completed_at,
       su.full_name AS sender_name,   su.phone AS sender_phone,
       ru.full_name AS receiver_name, ru.phone AS receiver_phone
FROM transactions t
LEFT JOIN wallets sw ON sw.id = t.sender_wallet_id
LEFT JOIN users  su ON sw.owner_type = 'user' AND su.id = sw.owner_id
LEFT JOIN wallets rw ON rw.id = t.receiver_wallet_id
LEFT JOIN users  ru ON rw.owner_type = 'user' AND ru.id = rw.owner_id
WHERE t.ref_code = $1;

-- name: CompleteTransaction :one
-- Guarded transition: only processing -> completed (fintech-review #5).
UPDATE transactions SET status = 'completed', completed_at = now()
WHERE id = $1 AND status = 'processing'
RETURNING id;

-- name: ListTransactionsByWallet :many
-- Keyset pagination: strictly older than cursor (created_at, id).
-- Counterparty identity joined for TxnListItem / recent contacts (m3-api-support);
-- masking happens in the service layer — raw names never leave internal/.
SELECT t.id, t.ref_code, t.type, t.status, t.sender_wallet_id, t.receiver_wallet_id,
       t.amount, t.fee, t.note, t.created_at, t.completed_at,
       su.full_name AS sender_name,   su.phone AS sender_phone,
       ru.full_name AS receiver_name, ru.phone AS receiver_phone
FROM transactions t
LEFT JOIN wallets sw ON sw.id = t.sender_wallet_id
LEFT JOIN users  su ON sw.owner_type = 'user' AND su.id = sw.owner_id
LEFT JOIN wallets rw ON rw.id = t.receiver_wallet_id
LEFT JOIN users  ru ON rw.owner_type = 'user' AND ru.id = rw.owner_id
WHERE (t.sender_wallet_id = $1 OR t.receiver_wallet_id = $1)
  AND (t.created_at, t.id) < ($2, $3::uuid)
ORDER BY t.created_at DESC, t.id DESC
LIMIT $4;

-- name: PendTransactionGuarded :exec
-- Guarded transition: only processing -> pending_approval (human gate, F6).
UPDATE transactions SET status = 'pending_approval'
WHERE id = $1 AND status = 'processing';

-- name: ResumeTransactionGuarded :exec
-- Guarded transition: only pending_approval -> processing (approval granted).
UPDATE transactions SET status = 'processing'
WHERE id = $1 AND status = 'pending_approval';

-- name: GetTransactionByWorkflow :one
-- Step handlers read the immutable money facts from the transaction row.
SELECT id, ref_code, status, type, sender_wallet_id, receiver_wallet_id, amount, fee, initiated_by
FROM transactions WHERE id = $1;

-- name: InsertReversalTransaction :one
-- Reversal is a passive posting (docs 04 §1): it does NOT run through the
-- workflow engine, so it is born 'completed'. reversal_of + the
-- one_reversal_per_txn unique index block a second reversal of the same txn.
INSERT INTO transactions
  (ref_code, idempotency_key, type, status, sender_wallet_id, receiver_wallet_id, amount, fee, note, initiated_by, reversal_of, completed_at)
VALUES ($1, $2, 'reversal', 'completed', $3, $4, $5, $6, $7, $8, $9, now())
RETURNING id, ref_code, created_at;

-- name: MarkTransactionReversed :execrows
-- Guarded completed -> reversed: the only transition allowed out of completed
-- (index.html §7), and the money invariant is preserved by the opposite ledger
-- entries the executor posts, not by mutating the original.
UPDATE transactions SET status = 'reversed'
WHERE id = $1 AND status = 'completed';

-- name: InsertPendingTopup :one
-- F10 E15: a topup awaiting the (mock) bank callback parks at 'created' —
-- no ledger legs yet; money moves only when the callback lands.
INSERT INTO transactions (ref_code, idempotency_key, type, status, sender_wallet_id, receiver_wallet_id, amount, fee, note, initiated_by)
VALUES ($1, $2, 'topup', 'created', $3, $4, $5, 0, $6, $7)
RETURNING id, ref_code, status, created_at;

-- name: GetTopupByRefCodeForUpdate :one
-- Serializes concurrent callbacks for the same ref_code (E15).
SELECT id, status, amount, sender_wallet_id, receiver_wallet_id
FROM transactions
WHERE ref_code = $1 AND type = 'topup'
FOR UPDATE;

-- name: CompleteTransactionFromCreated :execrows
-- Guarded transition: only created -> completed (bank callback path).
UPDATE transactions SET status = 'completed', completed_at = now()
WHERE id = $1 AND status = 'created';
