-- name: CountSenderTxnsSince :one
-- VELOCITY_5MIN: how many p2p transactions this sender started recently.
SELECT count(*)::int AS n
FROM transactions
WHERE initiated_by = $1 AND type = 'p2p' AND created_at >= $2;

-- name: AvgSenderAmount30d :one
-- AMOUNT_SPIKE baseline: the sender's historical COMPLETED p2p spend. Excludes
-- the in-flight transaction being evaluated (still 'processing' at FRAUD_CHECK
-- time) so a large amount spikes against history, not against itself. Returns
-- 0 when there's no history (new user).
SELECT COALESCE(AVG(amount), 0)::bigint AS avg_amount
FROM transactions
WHERE initiated_by = $1 AND type = 'p2p' AND status = 'completed'
  AND created_at >= now() - interval '30 days';

-- name: HasSentToReceiver :one
-- NEW_RECEIVER: has this sender ever paid this receiver wallet before?
SELECT EXISTS(
  SELECT 1 FROM transactions
  WHERE initiated_by = $1 AND receiver_wallet_id = $2
    AND type = 'p2p' AND status = 'completed'
)::boolean AS seen;

-- name: InsertFraudFlag :one
-- score stored as int 0-100 (::numeric cast keeps the column's type happy
-- without ever involving a Go float).
INSERT INTO fraud_flags (transaction_id, rule_code, score, reasons)
VALUES ($1, $2, sqlc.arg(score)::int, $3)
RETURNING id;

-- name: GetFraudFlagIDByTransaction :one
-- G19: the FRAUD_CHECK step may run twice for one transaction (redelivery); the second run
-- reuses the first run's flag instead of writing a duplicate.
SELECT id FROM fraud_flags WHERE transaction_id = $1 ORDER BY created_at, id LIMIT 1;

-- name: AppendFraudFlagReasons :exec
-- Advisory LLM text arrives after the decision (fire-and-forget) — merge it in.
UPDATE fraud_flags SET reasons = $2 WHERE id = $1;

-- name: ListFraudFlags :many
-- Open/resolved queue, paginated with optional ref_code search and date range.
SELECT f.id, f.transaction_id, t.ref_code, f.rule_code,
       f.score::int AS score, f.reasons, f.status, f.created_at
FROM fraud_flags f JOIN transactions t ON t.id = f.transaction_id
WHERE f.status = sqlc.arg('status')
  AND (sqlc.narg('ref_code')::text IS NULL OR t.ref_code ILIKE '%' || sqlc.narg('ref_code')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR f.created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR f.created_at <= sqlc.narg('to_at')::timestamptz)
ORDER BY
  CASE WHEN sqlc.arg('sort_order')::text = 'asc' THEN f.created_at END ASC,
  f.created_at DESC
LIMIT sqlc.arg('lim')::int OFFSET sqlc.arg('off')::int;

-- name: CountFraudFlags :one
SELECT COUNT(*)::bigint
FROM fraud_flags f JOIN transactions t ON t.id = f.transaction_id
WHERE f.status = sqlc.arg('status')
  AND (sqlc.narg('ref_code')::text IS NULL OR t.ref_code ILIKE '%' || sqlc.narg('ref_code')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR f.created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR f.created_at <= sqlc.narg('to_at')::timestamptz);

-- name: ListAllFraudFlags :many
-- Same as ListFraudFlags but across every status (the "all" tab).
SELECT f.id, f.transaction_id, t.ref_code, f.rule_code,
       f.score::int AS score, f.reasons, f.status, f.created_at
FROM fraud_flags f JOIN transactions t ON t.id = f.transaction_id
WHERE (sqlc.narg('ref_code')::text IS NULL OR t.ref_code ILIKE '%' || sqlc.narg('ref_code')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR f.created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR f.created_at <= sqlc.narg('to_at')::timestamptz)
ORDER BY
  CASE WHEN sqlc.arg('sort_order')::text = 'asc' THEN f.created_at END ASC,
  f.created_at DESC
LIMIT sqlc.arg('lim')::int OFFSET sqlc.arg('off')::int;

-- name: CountAllFraudFlags :one
SELECT COUNT(*)::bigint
FROM fraud_flags f JOIN transactions t ON t.id = f.transaction_id
WHERE (sqlc.narg('ref_code')::text IS NULL OR t.ref_code ILIKE '%' || sqlc.narg('ref_code')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR f.created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR f.created_at <= sqlc.narg('to_at')::timestamptz);


-- name: ResolveFraudFlagGuarded :execrows
-- Guarded open -> cleared|confirmed_fraud (fintech-review #5).
UPDATE fraud_flags SET status = $2, reviewer_id = $3, reviewed_at = now()
WHERE id = $1 AND status = 'open';

-- name: GetFraudFlag :one
SELECT id, transaction_id, status FROM fraud_flags WHERE id = $1;
