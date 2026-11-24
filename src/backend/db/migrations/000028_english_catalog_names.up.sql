-- English-only catalog: service, offer and action names that migrations 000003 and 000004 seeded in
-- Vietnamese. Existing databases are updated here; the old migrations stay as they were applied.
-- Rows are matched by their stable code, never by the old name, so a database that was edited by
-- an operator keeps its own text (only the seeded Vietnamese defaults are replaced).
UPDATE services SET name = 'P2P transfer'        WHERE code = 'P2P_TRANSFER'     AND name = 'Chuyển tiền P2P';
UPDATE services SET name = 'Request money & split bill' WHERE code = 'MONEY_REQUEST' AND name = 'Xin tiền & Split Bill';
UPDATE services SET name = 'Bank top-up'         WHERE code = 'TOPUP_BANK'       AND name = 'Nạp tiền Ngân hàng';
UPDATE services SET name = 'Corporate payroll'   WHERE code = 'ORG_DISBURSEMENT' AND name = 'Chi lương Doanh nghiệp';
UPDATE services SET name = 'Vault savings'       WHERE code = 'VAULT_SAVINGS'    AND name = 'Hũ tiết kiệm';

UPDATE services
SET actions = replace(replace(actions::text,
        'Hạch toán Sổ cái (GL Posting)', 'Ledger posting (GL Posting)'),
        'Gọi Gateway Ngân hàng Đối tác', 'Notify partner bank gateway')::jsonb
WHERE code = 'P2P_TRANSFER';

UPDATE offers SET name = 'Personal (KYC tier 1)'                   WHERE code = 'OFFER_CUST_L1'      AND name = 'Khách hàng Cá nhân (KYC Tier 1)';
UPDATE offers SET name = 'Personal (KYC tier 2)'                   WHERE code = 'OFFER_CUST_L2'      AND name = 'Khách hàng Cá nhân (KYC Tier 2)';
UPDATE offers SET name = 'Business Starter (Maker/Checker L1)'     WHERE code = 'OFFER_BIZ_STARTER'  AND name = 'Doanh nghiệp Starter (Maker/Checker L1)';
UPDATE offers SET name = 'Business Enterprise (Maker/Checker L2)'  WHERE code = 'OFFER_BIZ_ENT'      AND name = 'Doanh nghiệp Enterprise (Maker/Checker L2)';