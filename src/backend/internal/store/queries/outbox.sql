-- name: InsertOutbox :one
INSERT INTO outbox (aggregate, event_type, payload, available_at)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ClaimOutboxBatch :many
-- SKIP LOCKED: many relays can run concurrently without double-publishing.
SELECT id, event_type, payload FROM outbox
WHERE published_at IS NULL AND available_at <= now()
ORDER BY id LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxPublished :exec
UPDATE outbox SET published_at = now() WHERE id = ANY($1::bigint[]);
