-- name: InsertOrganization :one
INSERT INTO organizations (name, created_by) VALUES ($1, $2)
RETURNING id, name, status, created_at;

-- name: GetOrganization :one
SELECT id, name, status, created_at FROM organizations WHERE id = $1;

-- name: InsertOrgMember :exec
INSERT INTO org_members (org_id, user_id, role, checker_level)
VALUES ($1, $2, $3, sqlc.narg(checker_level));

-- name: GetOrgMember :one
-- AuthZ predicate lives in SQL, not just middleware (fintech-review #8).
SELECT role, COALESCE(checker_level, 0)::smallint AS checker_level
FROM org_members WHERE org_id = $1 AND user_id = $2;

-- name: ListUserOrganizations :many
SELECT o.id AS org_id, o.name, o.status, m.role, COALESCE(m.checker_level, 0)::smallint AS checker_level, w.id AS wallet_id, w.balance
FROM org_members m
JOIN organizations o ON o.id = m.org_id
LEFT JOIN wallets w ON w.owner_type = 'organization' AND w.owner_id = o.id
WHERE m.user_id = $1 AND o.status = 'active';

-- name: ListOrganizationsAdmin :many
-- Admin ops overview — every org (not just ones the caller belongs to, unlike
-- ListUserOrganizations which is scoped to the org-member's own JWT and can't
-- be reused for an admin-audience token). Paginated, same shared filter shape
-- as audit logs / workflow instances.
SELECT o.id AS org_id, o.name, o.status, o.created_at,
  (SELECT COUNT(*) FROM org_members m WHERE m.org_id = o.id)::bigint AS member_count,
  (SELECT COUNT(*) FROM approval_policies p WHERE p.org_id = o.id AND p.is_active)::bigint AS policies_count,
  (SELECT COUNT(*) FROM approval_requests ar WHERE ar.org_id = o.id AND ar.status = 'pending')::bigint AS pending_approvals_count
FROM organizations o
WHERE sqlc.narg('name')::text IS NULL OR o.name ILIKE '%' || sqlc.narg('name')::text || '%'
ORDER BY
  CASE WHEN sqlc.arg('sort_order')::text = 'asc' THEN o.created_at END ASC,
  o.created_at DESC
LIMIT sqlc.arg('lim')::int OFFSET sqlc.arg('off')::int;

-- name: CountOrganizationsAdmin :one
SELECT COUNT(*)::bigint
FROM organizations o
WHERE sqlc.narg('name')::text IS NULL OR o.name ILIKE '%' || sqlc.narg('name')::text || '%';
