-- name: InsertLedgerEntry :one
INSERT INTO ledger_entries (transaction_id, wallet_id, direction, amount, balance_after)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: SumLedgerByWallet :one
-- Recon helper: derived balance from the ledger (credit adds, debit subtracts).
SELECT COALESCE(SUM(CASE direction WHEN 'credit' THEN amount ELSE -amount END), 0)::bigint AS balance
FROM ledger_entries WHERE wallet_id = $1;
