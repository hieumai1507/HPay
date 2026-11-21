-- name: ListServices :many
-- Admin catalog list: optional name/code search, paginated. `name` is a
-- narg so an absent filter matches everything instead of matching '%%'.
SELECT * FROM services
WHERE (sqlc.narg('name')::text IS NULL
       OR name ILIKE '%' || sqlc.narg('name')::text || '%'
       OR code ILIKE '%' || sqlc.narg('name')::text || '%')
ORDER BY
  CASE WHEN sqlc.arg('sort_order')::text = 'asc' THEN name END ASC,
  name DESC
LIMIT sqlc.arg('lim')::int OFFSET sqlc.arg('off')::int;

-- name: CountServices :one
-- Must mirror ListServices' WHERE exactly or total_pages lies.
SELECT COUNT(*)::bigint FROM services
WHERE (sqlc.narg('name')::text IS NULL
       OR name ILIKE '%' || sqlc.narg('name')::text || '%'
       OR code ILIKE '%' || sqlc.narg('name')::text || '%');

-- name: GetService :one
SELECT * FROM services
WHERE id = $1;

-- name: GetServiceByCode :one
SELECT * FROM services
WHERE code = $1;

-- name: InsertService :one
INSERT INTO services (code, name, category, status, action_priority, channels, full_name, service_note, transaction_type, confirmation, service_target, amount_formula, actions, input_fields)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
RETURNING *;

-- name: UpdateServiceStatus :one
UPDATE services
SET status = $2
WHERE id = $1
RETURNING *;

-- name: UpdateService :one
UPDATE services
SET name = $2, category = $3, status = $4, action_priority = $5, channels = $6,
    full_name = $7, service_note = $8, transaction_type = $9, confirmation = $10,
    service_target = $11, amount_formula = $12, actions = $13, input_fields = $14
WHERE id = $1
RETURNING *;

-- name: ListOffers :many
SELECT 
    o.id, o.code, o.name, o.role, o.level, o.is_active, 
    o.primary_pocket_max, o.max_bank_accounts, o.updated_by, o.updated_at, o.created_at,
    COUNT(os.id)::int AS authorized_services_count
FROM offers o
LEFT JOIN offer_services os ON os.offer_id = o.id AND os.is_enabled = true
WHERE (sqlc.narg('name')::text IS NULL
       OR o.name ILIKE '%' || sqlc.narg('name')::text || '%'
       OR o.code ILIKE '%' || sqlc.narg('name')::text || '%')
GROUP BY o.id
ORDER BY o.level ASC, o.name ASC
LIMIT sqlc.arg('lim')::int OFFSET sqlc.arg('off')::int;

-- name: CountOffers :one
SELECT COUNT(*)::bigint FROM offers o
WHERE (sqlc.narg('name')::text IS NULL
       OR o.name ILIKE '%' || sqlc.narg('name')::text || '%'
       OR o.code ILIKE '%' || sqlc.narg('name')::text || '%');

-- name: GetOffer :one
SELECT * FROM offers
WHERE id = $1;

-- name: InsertOffer :one
INSERT INTO offers (code, name, role, level, is_active, primary_pocket_max, max_bank_accounts, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: UpdateOffer :one
UPDATE offers
SET name = $2, role = $3, level = $4, is_active = $5, primary_pocket_max = $6, max_bank_accounts = $7, updated_by = $8, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ListOfferServices :many
SELECT 
    os.id, os.offer_id, os.service_id, os.is_enabled, os.min_amount, os.max_amount_per_txn, 
    os.daily_limit, os.monthly_limit, os.interval_seconds,
    s.code AS service_code, s.name AS service_name
FROM offer_services os
JOIN services s ON s.id = os.service_id
WHERE os.offer_id = $1;

-- name: ListAuthMatrix :many
SELECT * FROM offer_auth_matrix
WHERE offer_service_id = $1
ORDER BY min_amount ASC;
