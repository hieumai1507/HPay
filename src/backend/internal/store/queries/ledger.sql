-- name: InsertLedgerEntry :one
INSERT INTO ledger_entries (transaction_id, wallet_id, direction, amount, balance_after)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: SumLedgerByWallet :one
-- Recon helper: derived balance from the ledger (credit adds, debit subtracts).
SELECT COALESCE(SUM(CASE direction WHEN 'credit' THEN amount ELSE -amount END), 0)::bigint AS balance
FROM ledger_entries WHERE wallet_id = $1;

-- name: InsertLedgerPosting :execrows
-- ADR-0010 guard. Zero rows affected means this (transaction, kind) was already
-- posted; the caller decides whether that is a replay or a conflicting payload.
INSERT INTO ledger_postings (transaction_id, kind, total_amount)
VALUES ($1, $2, $3)
ON CONFLICT (transaction_id, kind) DO NOTHING;

-- name: GetLedgerPosting :one
SELECT transaction_id, kind, total_amount, created_at
FROM ledger_postings WHERE transaction_id = $1 AND kind = $2;
