-- name: InsertApprovalPolicy :one
INSERT INTO approval_policies (org_id, name, min_amount, max_amount, tiers, version, created_by)
VALUES ($1, $2, $3, sqlc.narg(max_amount), $4, $5, $6)
RETURNING id, version;

-- name: DeactivateOrgPoliciesByName :exec
UPDATE approval_policies SET is_active = false
WHERE org_id = $1 AND name = $2 AND is_active;

-- name: GetNextPolicyVersion :one
SELECT COALESCE(MAX(version), 0)::int + 1 AS next_version
FROM approval_policies WHERE org_id = $1 AND name = $2;

-- name: GetActivePolicyForAmount :one
-- Amount-range match; newest version wins when ranges overlap.
SELECT id, name, min_amount, max_amount, tiers, version
FROM approval_policies
WHERE org_id = $1 AND is_active
  AND min_amount <= sqlc.arg(amount)
  AND (max_amount IS NULL OR sqlc.arg(amount) <= max_amount)
ORDER BY version DESC
LIMIT 1;

-- name: InsertApprovalRequest :one
-- org_id / instance_id / policy_id are NULL for non-org subjects (reversal,
-- account_lock, policy_change): pass uuid.Nil and NULLIF maps it to NULL so
-- the nullable FKs are satisfied. The org path passes real ids unaffected.
INSERT INTO approval_requests
  (subject_type, subject_id, org_id, instance_id, policy_id, policy_snapshot, created_by, expires_at)
VALUES (
  $1, $2,
  NULLIF(sqlc.arg(org_id)::uuid, '00000000-0000-0000-0000-000000000000'::uuid),
  NULLIF(sqlc.arg(instance_id)::uuid, '00000000-0000-0000-0000-000000000000'::uuid),
  NULLIF(sqlc.arg(policy_id)::uuid, '00000000-0000-0000-0000-000000000000'::uuid),
  $3, $4, $5)
RETURNING id;

-- name: ExistsPendingReversalForTxn :one
SELECT EXISTS(
  SELECT 1 FROM approval_requests
  WHERE subject_type = 'reversal' AND subject_id = $1 AND status = 'pending'
)::boolean AS exists;

-- name: ListPendingAdminApprovals :many
-- Admin "My Tasks": pending requests whose subject is an admin operation
-- (reversal / account_lock / policy_change), with the target transaction's
-- money facts when the subject is a reversal.
SELECT r.id, r.subject_type, r.subject_id, r.current_level,
       r.created_by, r.expires_at, r.created_at,
       COALESCE(t.ref_code, '') AS ref_code,
       COALESCE(t.amount, 0)::bigint AS amount,
       COALESCE(t.fee, 0)::bigint AS fee,
       COALESCE(r.reason, '') AS reason
FROM approval_requests r
LEFT JOIN transactions t ON t.id = r.subject_id AND r.subject_type = 'reversal'
WHERE r.status = 'pending' AND r.expires_at > now()
  AND r.subject_type <> 'org_transaction'
  AND (sqlc.narg('ref_code')::text IS NULL OR t.ref_code ILIKE '%' || sqlc.narg('ref_code')::text || '%')
ORDER BY r.created_at
LIMIT sqlc.arg('lim')::int OFFSET sqlc.arg('off')::int;

-- name: CountPendingAdminApprovals :one
SELECT COUNT(*)::bigint
FROM approval_requests r
LEFT JOIN transactions t ON t.id = r.subject_id AND r.subject_type = 'reversal'
WHERE r.status = 'pending' AND r.expires_at > now()
  AND r.subject_type <> 'org_transaction'
  AND (sqlc.narg('ref_code')::text IS NULL OR t.ref_code ILIKE '%' || sqlc.narg('ref_code')::text || '%');

-- name: GetApprovalRequest :one
SELECT id, subject_type, subject_id,
       COALESCE(org_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS org_id,
       COALESCE(instance_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS instance_id,
       policy_snapshot, status, current_level, created_by, expires_at, created_at
FROM approval_requests WHERE id = $1;

-- name: GetApprovalRequestForUpdate :one
-- E08: concurrent checkers serialize on this row lock.
SELECT id, subject_type, subject_id,
       COALESCE(org_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS org_id,
       COALESCE(instance_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS instance_id,
       policy_snapshot, status, current_level, created_by, expires_at
FROM approval_requests WHERE id = $1 FOR UPDATE;

-- name: InsertApprovalAction :one
INSERT INTO approval_actions (request_id, checker_id, level, action)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ListApprovalActions :many
SELECT checker_id, level, action, acted_at
FROM approval_actions WHERE request_id = $1 ORDER BY id;

-- name: ListApprovalActionsWithNames :many
-- Signature history for the detail endpoint: same rows as ListApprovalActions
-- plus the checker's display name (frame 21's "CHỮ KÝ" (signatures) list).
SELECT a.checker_id, u.full_name AS checker_name, a.level, a.action, a.acted_at
FROM approval_actions a
JOIN users u ON u.id = a.checker_id
WHERE a.request_id = $1 ORDER BY a.id;

-- name: GetApprovalRequestDetail :one
-- Detail view for GET /v1/org/approvals/{id} (maker or org member). Scoped to
-- subject_type = org_transaction — the only subject the mobile app shows.
SELECT r.id, r.subject_type, r.subject_id, r.org_id, o.name AS org_name,
       r.policy_snapshot, r.status, r.current_level,
       r.created_by, u.full_name AS maker_name,
       r.expires_at, r.created_at, r.finalized_at,
       t.ref_code, t.amount, t.fee
FROM approval_requests r
JOIN organizations o ON o.id = r.org_id
JOIN users u ON u.id = r.created_by
JOIN transactions t ON t.id = r.subject_id AND r.subject_type = 'org_transaction'
WHERE r.id = $1;

-- name: GetApprovalRequestDetailByRefCode :one
-- Same shape as GetApprovalRequestDetail, keyed by the transaction ref_code
-- CreateDisbursement returns synchronously — the approval_request row itself
-- is only created later by the pause hook once the engine reaches
-- WAIT_APPROVAL, so the maker's client cannot know its id right after
-- creating the disbursement (docs plan maker-checker-detail-screens).
SELECT r.id, r.subject_type, r.subject_id, r.org_id, o.name AS org_name,
       r.policy_snapshot, r.status, r.current_level,
       r.created_by, u.full_name AS maker_name,
       r.expires_at, r.created_at, r.finalized_at,
       t.ref_code, t.amount, t.fee
FROM approval_requests r
JOIN organizations o ON o.id = r.org_id
JOIN users u ON u.id = r.created_by
JOIN transactions t ON t.id = r.subject_id AND r.subject_type = 'org_transaction'
WHERE t.ref_code = $1;

-- name: HasApprovalAction :one
SELECT EXISTS(
  SELECT 1 FROM approval_actions WHERE request_id = $1 AND checker_id = $2
)::boolean AS acted;

-- name: SetApprovalCurrentLevel :exec
-- Guarded: only pending requests advance their level (fintech-review #5).
UPDATE approval_requests SET current_level = $2
WHERE id = $1 AND status = 'pending';

-- name: FinalizeApprovalRequest :one
-- Guarded transition pending -> approved|rejected|expired. The WHERE clause is
-- the E08/E10 claim: whoever loses the race gets no row back.
UPDATE approval_requests SET status = $2, finalized_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING id, subject_type, subject_id,
  COALESCE(instance_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS instance_id;

-- name: ListDueApprovalIDs :many
SELECT id FROM approval_requests
WHERE status = 'pending' AND expires_at < now()
ORDER BY expires_at
LIMIT $1;

-- name: ListPendingApprovalsForChecker :many
-- "My Tasks": pending requests in my orgs whose current tier matches my
-- checker level and which I have not acted on yet.
SELECT r.id, r.subject_type, r.subject_id, o.id AS org_id, o.name AS org_name,
       r.current_level, r.expires_at, r.created_at,
       t.ref_code, t.amount, t.fee, u.full_name AS maker_name
FROM approval_requests r
JOIN organizations o ON o.id = r.org_id
JOIN org_members m ON m.org_id = r.org_id AND m.user_id = sqlc.arg(checker_id)
  AND m.role = 'checker' AND m.checker_level = r.current_level
JOIN transactions t ON t.id = r.subject_id AND r.subject_type = 'org_transaction'
JOIN users u ON u.id = r.created_by
WHERE r.status = 'pending' AND r.expires_at > now()
  AND NOT EXISTS (
    SELECT 1 FROM approval_actions a
    WHERE a.request_id = r.id AND a.checker_id = sqlc.arg(checker_id))
ORDER BY r.created_at;

-- name: MarkApprovalNeedsManual :one
-- ADR-0013: a request whose effect could not be applied (for example a reversal
-- whose receiver no longer holds the money) is parked instead of staying pending.
-- Guarded pending -> needs_manual; whoever loses the race gets no row.
UPDATE approval_requests SET status = 'needs_manual', finalized_at = now()
WHERE id = $1 AND status = 'pending'
RETURNING id;
