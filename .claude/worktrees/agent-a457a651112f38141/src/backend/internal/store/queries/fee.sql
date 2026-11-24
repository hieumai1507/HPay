-- name: GetFeePolicy :one
-- Tier-specific policy wins over the NULL-tier default; newest version wins.
SELECT id, fixed_fee, percent_bp, min_fee, max_fee
FROM fee_policies
WHERE txn_type = $1 AND is_active
  AND (kyc_status = sqlc.arg(kyc_tier)::kyc_status OR kyc_status IS NULL)
ORDER BY (kyc_status IS NOT NULL) DESC, version DESC
LIMIT 1;
