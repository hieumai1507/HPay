-- name: InsertWorkflowDefinition :one
INSERT INTO workflow_definitions (name, version, steps, is_active)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: GetActiveDefinitionByName :one
SELECT id, name, version, steps FROM workflow_definitions
WHERE name = $1 AND is_active;

-- name: GetDefinitionByID :one
SELECT id, name, version, steps FROM workflow_definitions WHERE id = $1;

-- name: InsertWorkflowInstance :one
INSERT INTO workflow_instances (definition_id, transaction_id, status, current_step, context)
VALUES (
    sqlc.arg(definition_id),
    NULLIF(sqlc.arg(transaction_id)::uuid, '00000000-0000-0000-0000-000000000000'::uuid),
    'running',
    sqlc.arg(current_step),
    sqlc.arg(context)
)
RETURNING id;

-- name: GetInstanceForUpdate :one
SELECT id, definition_id,
       COALESCE(transaction_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS transaction_id,
       status, current_step, context
FROM workflow_instances WHERE id = $1 FOR UPDATE;

-- name: GetInstance :one
SELECT id, definition_id,
       COALESCE(transaction_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS transaction_id,
       status, current_step, context, created_at, updated_at
FROM workflow_instances WHERE id = $1;

-- name: AdvanceInstance :exec
UPDATE workflow_instances
SET current_step = $2, status = $3, context = $4, updated_at = now()
WHERE id = $1;

-- name: InsertStepExecution :one
INSERT INTO step_executions (instance_id, step_name, attempt, status, input, output, error, finished_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
RETURNING id;

-- name: GetStepExecution :one
SELECT id, status, output, error FROM step_executions
WHERE instance_id = $1 AND step_name = $2 AND attempt = $3;

-- name: ListStepExecutions :many
SELECT id, step_name, attempt, status, input, output, error, started_at, finished_at
FROM step_executions WHERE instance_id = $1 ORDER BY id;

-- name: ClaimStuckInstances :many
-- Lease pattern (E20, borrowed from TMDS pending->attempt): the UPDATE is the claim — a second
-- sweeper running in parallel cannot match an already-claimed row.
UPDATE workflow_instances w
SET status = 'recovering', updated_at = now()
WHERE w.id IN (
    SELECT i.id FROM workflow_instances i
    WHERE i.status = 'running' AND i.updated_at < $1
    ORDER BY i.updated_at LIMIT $2
    FOR UPDATE SKIP LOCKED
)
RETURNING w.id, w.definition_id,
    COALESCE(w.transaction_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS transaction_id,
    w.current_step, w.context;

-- name: ResumeRecoveredInstance :exec
UPDATE workflow_instances SET status = 'running', updated_at = now()
WHERE id = $1 AND status = 'recovering';

-- name: FailTransactionGuarded :exec
-- Only non-terminal states may move to failed (fintech-review #5).
UPDATE transactions SET status = 'failed', failure_reason = $2
WHERE id = $1 AND status IN ('created', 'processing', 'pending_approval', 'processing_async');

-- name: GetInstanceByTransactionID :one
SELECT id, status, current_step FROM workflow_instances WHERE transaction_id = $1;

-- name: ListWorkflowInstances :many
-- Admin Workflow Monitor: status/definition/stuck_only mirror the 3 controls
-- already on WorkflowPage.tsx (docs plan workflow-instances-filters). ref_code
-- search + created_at range reuse the same listing.Filter shape as every
-- other admin list.
SELECT i.id, i.definition_id,
       COALESCE(i.transaction_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS transaction_id,
       i.status, i.current_step, i.context, i.created_at, i.updated_at,
       d.name AS definition_name, d.version AS definition_version,
       t.ref_code AS txn_ref, t.amount AS txn_amount
FROM workflow_instances i
JOIN workflow_definitions d ON d.id = i.definition_id
LEFT JOIN transactions t ON t.id = i.transaction_id
WHERE (sqlc.narg('status')::text IS NULL OR i.status::text = sqlc.narg('status')::text)
  AND (sqlc.narg('definition')::text IS NULL OR d.name ILIKE '%' || sqlc.narg('definition')::text || '%')
  AND (NOT sqlc.arg('stuck_only')::bool OR (i.status = 'running' AND i.created_at < now() - interval '5 minutes'))
  AND (sqlc.narg('name')::text IS NULL OR t.ref_code ILIKE '%' || sqlc.narg('name')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR i.created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR i.created_at <= sqlc.narg('to_at')::timestamptz)
ORDER BY
  CASE WHEN sqlc.arg('sort_order')::text = 'asc' THEN i.created_at END ASC,
  i.created_at DESC
LIMIT sqlc.arg('lim')::int OFFSET sqlc.arg('off')::int;

-- name: CountWorkflowInstances :one
-- Full row count so total_pages reflects the filtered set, not just the page.
SELECT COUNT(*)::bigint
FROM workflow_instances i
JOIN workflow_definitions d ON d.id = i.definition_id
LEFT JOIN transactions t ON t.id = i.transaction_id
WHERE (sqlc.narg('status')::text IS NULL OR i.status::text = sqlc.narg('status')::text)
  AND (sqlc.narg('definition')::text IS NULL OR d.name ILIKE '%' || sqlc.narg('definition')::text || '%')
  AND (NOT sqlc.arg('stuck_only')::bool OR (i.status = 'running' AND i.created_at < now() - interval '5 minutes'))
  AND (sqlc.narg('name')::text IS NULL OR t.ref_code ILIKE '%' || sqlc.narg('name')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR i.created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR i.created_at <= sqlc.narg('to_at')::timestamptz);

