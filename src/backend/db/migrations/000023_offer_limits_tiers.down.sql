-- Users on L0 go back to the old backfill offer (L2); limits go back to the 000006 placeholders.
UPDATE user_offers uo SET offer_id = l2.id, assigned_by = 'system', assigned_at = now()
FROM offers l0, offers l2
WHERE l0.code = 'OFFER_CUST_L0' AND l2.code = 'OFFER_CUST_L2' AND uo.offer_id = l0.id;

DELETE FROM offers WHERE code = 'OFFER_CUST_L0' AND NOT EXISTS (SELECT 1 FROM user_offers uo WHERE uo.offer_id = offers.id);

UPDATE offer_services os
SET max_amount_per_txn = 400000, daily_limit = 2000000, monthly_limit = 8000000
FROM offers o WHERE o.id = os.offer_id AND o.code IN ('OFFER_CUST_L1', 'OFFER_CUST_L2');

UPDATE offers SET primary_pocket_max = 200000 WHERE code = 'OFFER_CUST_L1';
UPDATE offers SET primary_pocket_max = 800000 WHERE code = 'OFFER_CUST_L2';