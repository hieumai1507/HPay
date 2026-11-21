-- Offer limits by tier (docs/specs/w3-offer-limits). Amounts: approved USD table x 25,000 VND/USD.
INSERT INTO offers (id, code, name, role, level, is_active, primary_pocket_max, max_bank_accounts, updated_by)
VALUES ('b0000000-0000-0000-0000-000000000000', 'OFFER_CUST_L0', 'Personal (unverified)', 'customer', 0, true, 6250000, 1, 'policy-2026-11-21')
ON CONFLICT (code) DO NOTHING;

-- L0 offers the same services as L1, with its own limits set below.
INSERT INTO offer_services (offer_id, service_id, is_enabled, min_amount, max_amount_per_txn, daily_limit, monthly_limit, interval_seconds)
SELECT l0.id, os.service_id, os.is_enabled, os.min_amount, 0, 0, 0, os.interval_seconds
FROM offers l0
JOIN offers l1 ON l1.code = 'OFFER_CUST_L1'
JOIN offer_services os ON os.offer_id = l1.id
WHERE l0.code = 'OFFER_CUST_L0'
ON CONFLICT (offer_id, service_id) DO NOTHING;

UPDATE offer_services os
SET max_amount_per_txn = v.per_txn, daily_limit = v.daily, monthly_limit = v.monthly
FROM offers o,
     (VALUES ('OFFER_CUST_L0', 2500000::bigint,   2500000::bigint,   25000000::bigint),
             ('OFFER_CUST_L1', 62500000::bigint,  125000000::bigint, 625000000::bigint),
             ('OFFER_CUST_L2', 250000000::bigint, 375000000::bigint, 2500000000::bigint)) AS v(code, per_txn, daily, monthly)
WHERE o.id = os.offer_id AND o.code = v.code;

UPDATE offers o SET primary_pocket_max = v.ceiling, updated_by = 'policy-2026-11-21', updated_at = now()
FROM (VALUES ('OFFER_CUST_L0', 6250000::bigint), ('OFFER_CUST_L1', 250000000::bigint), ('OFFER_CUST_L2', 1250000000::bigint)) AS v(code, ceiling)
WHERE o.code = v.code;

-- Same PIN / OTP / 2FA matrix as the other customer offers, for the new rows only.
INSERT INTO offer_auth_matrix (offer_service_id, min_amount, max_amount, auth_method)
SELECT os.id, 0, 2000000, 'pin' FROM offer_services os JOIN offers o ON o.id = os.offer_id
  WHERE o.code = 'OFFER_CUST_L0' AND NOT EXISTS (SELECT 1 FROM offer_auth_matrix m WHERE m.offer_service_id = os.id AND m.auth_method = 'pin');
INSERT INTO offer_auth_matrix (offer_service_id, min_amount, max_amount, auth_method)
SELECT os.id, 2000000, 50000000, 'otp' FROM offer_services os JOIN offers o ON o.id = os.offer_id
  WHERE o.code = 'OFFER_CUST_L0' AND NOT EXISTS (SELECT 1 FROM offer_auth_matrix m WHERE m.offer_service_id = os.id AND m.auth_method = 'otp');
INSERT INTO offer_auth_matrix (offer_service_id, min_amount, max_amount, auth_method)
SELECT os.id, 50000000, NULL, '2fa' FROM offer_services os JOIN offers o ON o.id = os.offer_id
  WHERE o.code = 'OFFER_CUST_L0' AND NOT EXISTS (SELECT 1 FROM offer_auth_matrix m WHERE m.offer_service_id = os.id AND m.auth_method = '2fa');

-- Users still on the system-assigned L2 backfill (000006) are mapped by their KYC status.
UPDATE user_offers uo
SET offer_id = CASE WHEN u.kyc_status IN ('verified', 'auto_approved') THEN l1.id ELSE l0.id END,
    assigned_by = 'policy-2026-11-21', assigned_at = now()
FROM users u, offers cur, offers l0, offers l1
WHERE u.id = uo.user_id AND cur.id = uo.offer_id AND cur.code = 'OFFER_CUST_L2' AND uo.assigned_by = 'system'
  AND l0.code = 'OFFER_CUST_L0' AND l1.code = 'OFFER_CUST_L1';