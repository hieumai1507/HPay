-- name: ListAuditLogs :many
-- Admin audit trail, paginated with optional free-text search (action/resource/
-- reason) and a created_at date range — same shared filter shape as fraud
-- flags and workflow instances (docs plan audit-log-search).
SELECT *
FROM audit_logs
WHERE (sqlc.narg('name')::text IS NULL
       OR action ILIKE '%' || sqlc.narg('name')::text || '%'
       OR resource ILIKE '%' || sqlc.narg('name')::text || '%'
       OR reason ILIKE '%' || sqlc.narg('name')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR created_at <= sqlc.narg('to_at')::timestamptz)
ORDER BY
  CASE WHEN sqlc.arg('sort_order')::text = 'asc' THEN created_at END ASC,
  created_at DESC
LIMIT sqlc.arg('lim')::int OFFSET sqlc.arg('off')::int;

-- name: CountAuditLogs :one
SELECT COUNT(*)::bigint
FROM audit_logs
WHERE (sqlc.narg('name')::text IS NULL
       OR action ILIKE '%' || sqlc.narg('name')::text || '%'
       OR resource ILIKE '%' || sqlc.narg('name')::text || '%'
       OR reason ILIKE '%' || sqlc.narg('name')::text || '%')
  AND (sqlc.narg('from_at')::timestamptz IS NULL OR created_at >= sqlc.narg('from_at')::timestamptz)
  AND (sqlc.narg('to_at')::timestamptz IS NULL OR created_at <= sqlc.narg('to_at')::timestamptz);
