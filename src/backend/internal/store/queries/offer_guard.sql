-- name: GetOfferServiceForUser :one
-- Resolve the caller's Offer package and its config for one service code.
-- No row = the service is not part of the user's offer (OfferGuard fails closed).
SELECT os.id, os.offer_id, os.service_id, os.is_enabled, os.min_amount,
       os.max_amount_per_txn, os.daily_limit, os.monthly_limit, os.interval_seconds
FROM user_offers uo
JOIN offers o ON o.id = uo.offer_id AND o.is_active
JOIN offer_services os ON os.offer_id = uo.offer_id
JOIN services s ON s.id = os.service_id
WHERE uo.user_id = $1 AND s.code = $2;

-- name: GetOfferServiceByOfferCode :one
-- Fallback config when a user has no explicit offer assignment: the platform
-- default package. Keeps every transaction metered instead of unlimited.
SELECT os.id, os.offer_id, os.service_id, os.is_enabled, os.min_amount,
       os.max_amount_per_txn, os.daily_limit, os.monthly_limit, os.interval_seconds
FROM offers o
JOIN offer_services os ON os.offer_id = o.id
JOIN services s ON s.id = os.service_id
WHERE o.code = $1 AND s.code = $2 AND o.is_active;

-- name: SumUserTxnAmountSince :one
-- Rolling volume for daily/monthly limits. Failed/reversed do not consume quota.
SELECT COALESCE(SUM(amount + fee), 0)::bigint AS total
FROM transactions
WHERE initiated_by = $1
  AND type = $2
  AND status NOT IN ('failed', 'reversed')
  AND created_at >= $3;

-- name: LastUserTxnAt :one
-- Most recent attempt of this type, for the interval_seconds throttle.
SELECT COALESCE(MAX(created_at), 'epoch'::timestamptz)::timestamptz AS last_at
FROM transactions
WHERE initiated_by = $1 AND type = $2;

-- name: AssignUserOffer :exec
INSERT INTO user_offers (user_id, offer_id, assigned_by)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
  SET offer_id = EXCLUDED.offer_id, assigned_by = EXCLUDED.assigned_by, assigned_at = now();
