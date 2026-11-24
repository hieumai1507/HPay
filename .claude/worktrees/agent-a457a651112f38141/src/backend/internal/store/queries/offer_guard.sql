-- name: GetOfferServiceForUser :one
-- Resolve the caller's Offer package and its config for one service code.
-- No row = the service is not part of the user's offer (OfferGuard fails closed).
-- An inactive offer is returned with offer_active = false so the guard can block it.
SELECT os.id, os.offer_id, os.service_id, os.is_enabled, os.min_amount,
       os.max_amount_per_txn, os.daily_limit, os.monthly_limit, os.interval_seconds,
       o.is_active AS offer_active, o.code AS offer_code
FROM user_offers uo
JOIN offers o ON o.id = uo.offer_id
JOIN offer_services os ON os.offer_id = uo.offer_id
JOIN services s ON s.id = os.service_id
WHERE uo.user_id = $1 AND s.code = $2;

-- name: GetOfferServiceByOfferCode :one
-- Fallback config when a user has no explicit offer assignment: the platform
-- default package. Keeps every transaction metered instead of unlimited.
SELECT os.id, os.offer_id, os.service_id, os.is_enabled, os.min_amount,
       os.max_amount_per_txn, os.daily_limit, os.monthly_limit, os.interval_seconds,
       o.is_active AS offer_active, o.code AS offer_code
FROM offers o
JOIN offer_services os ON os.offer_id = o.id
JOIN services s ON s.id = os.service_id
WHERE o.code = $1 AND s.code = $2;

-- name: SumUserTxnAmountSince :one
-- Rolling volume for daily/monthly limits. Failed, reversed and awaiting_auth (not yet confirmed) do not consume quota.
SELECT COALESCE(SUM(amount + fee), 0)::bigint AS total
FROM transactions
WHERE initiated_by = $1
  AND type = $2
  AND status NOT IN ('failed', 'reversed', 'awaiting_auth')
  AND created_at >= $3;

-- name: LastUserTxnAt :one
-- Most recent attempt of this type, for the interval_seconds throttle.
SELECT COALESCE(MAX(created_at), 'epoch'::timestamptz)::timestamptz AS last_at
FROM transactions
WHERE initiated_by = $1 AND type = $2 AND status <> 'awaiting_auth';

-- name: AssignUserOffer :exec
INSERT INTO user_offers (user_id, offer_id, assigned_by)
VALUES ($1, $2, $3)
ON CONFLICT (user_id) DO UPDATE
  SET offer_id = EXCLUDED.offer_id, assigned_by = EXCLUDED.assigned_by, assigned_at = now();

-- name: HasUserOffer :one
SELECT EXISTS (SELECT 1 FROM user_offers WHERE user_id = $1) AS assigned;

-- name: AssignKycApprovedOffer :exec
-- KYC approval (ADR-0007, docs/specs/w3-offer-limits): L0 (or no offer yet) moves to L1.
-- L1 and L2 are kept; a merchant offer is never touched.
INSERT INTO user_offers (user_id, offer_id, assigned_by)
SELECT sqlc.arg('user_id')::uuid, o.id, 'kyc' FROM offers o WHERE o.code = 'OFFER_CUST_L1'
ON CONFLICT (user_id) DO UPDATE
  SET offer_id = EXCLUDED.offer_id, assigned_by = 'kyc', assigned_at = now()
  WHERE user_offers.offer_id IN (SELECT id FROM offers WHERE code = 'OFFER_CUST_L0');

-- name: AssignKycRejectedOffer :exec
-- KYC rejection: any customer offer goes back to L0. Merchant offers are never touched.
INSERT INTO user_offers (user_id, offer_id, assigned_by)
SELECT sqlc.arg('user_id')::uuid, o.id, 'kyc' FROM offers o WHERE o.code = 'OFFER_CUST_L0'
ON CONFLICT (user_id) DO UPDATE
  SET offer_id = EXCLUDED.offer_id, assigned_by = 'kyc', assigned_at = now()
  WHERE user_offers.offer_id IN (SELECT id FROM offers WHERE role = 'customer');
