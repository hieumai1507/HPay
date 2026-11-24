-- name: InsertKYCRequest :one
INSERT INTO kyc_requests (user_id, id_card_url, selfie_url)
VALUES ($1, $2, $3)
RETURNING id, status, created_at;

-- name: GetKYCRequest :one
-- ai_score is written as int 0-100; -1 means "not scored yet".
SELECT id, user_id, id_card_url, selfie_url,
       COALESCE(ai_score, -1)::int AS ai_score,
       COALESCE(ai_verdict, '') AS ai_verdict,
       ai_reasons, status,
       COALESCE(reviewer_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS reviewer_id,
       created_at
FROM kyc_requests WHERE id = $1;

-- name: GetLatestKYCByUser :one
SELECT id, status,
       COALESCE(ai_score, -1)::int AS ai_score,
       ai_reasons, created_at
FROM kyc_requests WHERE user_id = $1
ORDER BY created_at DESC LIMIT 1;

-- name: SetKYCScore :exec
-- Score is int 0-100 end-to-end; the ::int cast keeps the generated param an
-- int32 instead of pgtype.Numeric.
UPDATE kyc_requests SET ai_score = sqlc.arg(ai_score)::int, ai_verdict = $2, ai_reasons = $3
WHERE id = $1;

-- name: ClassifyKYCGuarded :execrows
-- Guarded pending -> auto_approved|manual_review|rejected: replaying the
-- CLASSIFY step (at-least-once delivery) is a no-op (fintech-review #5).
UPDATE kyc_requests SET status = $2
WHERE id = $1 AND status = 'pending';

-- name: ReviewKYCGuarded :execrows
-- Guarded manual_review -> verified|rejected. Two reviewers racing: the
-- second one matches 0 rows -> 409 (groundwork for admin edge A01).
UPDATE kyc_requests SET status = $2, reviewer_id = $3, reviewed_at = now()
WHERE id = $1 AND status = 'manual_review';

-- name: ListKYCQueue :many
SELECT k.id, k.user_id, u.full_name, u.phone,
       k.id_card_url, k.selfie_url,
       COALESCE(k.ai_score, -1)::int AS ai_score,
       COALESCE(k.ai_verdict, '') AS ai_verdict,
       k.ai_reasons, k.status, k.created_at
FROM kyc_requests k JOIN users u ON u.id = k.user_id
WHERE k.status = sqlc.arg('status')
  AND (sqlc.narg('name')::text IS NULL
       OR u.full_name ILIKE '%' || sqlc.narg('name')::text || '%'
       OR u.phone ILIKE '%' || sqlc.narg('name')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR k.created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR k.created_at <= sqlc.narg('to_at')::timestamptz)
ORDER BY
  CASE WHEN sqlc.arg('sort_order')::text = 'desc' THEN k.created_at END DESC,
  k.created_at ASC
LIMIT sqlc.arg('lim')::int OFFSET sqlc.arg('off')::int;

-- name: CountKYCQueue :one
SELECT COUNT(*)::bigint
FROM kyc_requests k JOIN users u ON u.id = k.user_id
WHERE k.status = sqlc.arg('status')
  AND (sqlc.narg('name')::text IS NULL
       OR u.full_name ILIKE '%' || sqlc.narg('name')::text || '%'
       OR u.phone ILIKE '%' || sqlc.narg('name')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR k.created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR k.created_at <= sqlc.narg('to_at')::timestamptz);

-- name: MarkUserKycPending :execrows
-- Only fresh or previously-rejected users may (re)enter KYC.
UPDATE users SET kyc_status = 'pending', updated_at = now()
WHERE id = $1 AND kyc_status IN ('none', 'rejected');

-- name: SetUserKycFromPending :execrows
-- CLASSIFY outcome: pending -> auto_approved | manual_review | rejected.
UPDATE users SET kyc_status = $2, updated_at = now()
WHERE id = $1 AND kyc_status = 'pending';

-- name: SetUserKycFromManualReview :execrows
-- Admin review outcome: manual_review -> verified | rejected.
UPDATE users SET kyc_status = $2, updated_at = now()
WHERE id = $1 AND kyc_status = 'manual_review';
