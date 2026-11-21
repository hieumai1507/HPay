-- name: ListCompletedTopupsByDate :many
-- Reconcile topups (the flows that touch the mock bank) for a given calendar
-- day, by the internal completion timestamp.
SELECT id, ref_code, amount, completed_at
FROM transactions
WHERE type = 'topup' AND status = 'completed'
  AND completed_at >= sqlc.arg(day)::date
  AND completed_at <  (sqlc.arg(day)::date + interval '1 day')
ORDER BY completed_at;

-- name: ListStatementsByRefCodes :many
-- E16: match by ref_code GLOBALLY, not by date — a statement that landed on a
-- neighbouring day still matches its transaction. Duplicates (same ref twice)
-- come back as separate rows so the caller can flag them.
SELECT id, ref_code, amount, stmt_date
FROM bank_statements
WHERE ref_code = ANY($1::varchar[])
ORDER BY ref_code, id;

-- name: ListStatementsByDate :many
-- Statements booked ON the run day — used to find missing_internal (a bank
-- entry with no matching internal transaction). Cutoff-shifted statements
-- (dated a neighbouring day) are found via ListStatementsByRefCodes instead.
SELECT id, ref_code, amount, stmt_date
FROM bank_statements WHERE stmt_date = $1
ORDER BY id;

-- name: InsertReconRun :one
INSERT INTO reconciliation_runs (run_date, total, matched, mismatched, ai_summary, report_url)
VALUES ($1, $2, $3, $4, sqlc.narg(ai_summary), sqlc.narg(report_url))
RETURNING id, run_date, created_at;

-- name: InsertReconItem :exec
INSERT INTO reconciliation_items (run_id, transaction_id, statement_id, result, detail)
VALUES (
  $1,
  NULLIF(sqlc.arg(transaction_id)::uuid, '00000000-0000-0000-0000-000000000000'::uuid),
  sqlc.narg(statement_id),
  $2, $3
);

-- name: GetReconRunByDate :one
SELECT id, run_date, total, matched, mismatched,
       COALESCE(ai_summary, '') AS ai_summary,
       COALESCE(report_url, '') AS report_url,
       created_at
FROM reconciliation_runs WHERE run_date = $1;

-- name: ListReconItems :many
SELECT r.id,
       COALESCE(r.transaction_id, '00000000-0000-0000-0000-000000000000'::uuid)::uuid AS transaction_id,
       r.statement_id, r.result, r.detail
FROM reconciliation_items r
JOIN reconciliation_runs run ON run.id = r.run_id
WHERE run.run_date = $1
ORDER BY r.id;

-- name: ListWalletLedgerMismatches :many
-- Internal reconciliation: wallets.balance must equal SUM(ledger_entries).
-- Any row returned is a broken invariant (should always be empty).
SELECT w.id AS wallet_id, w.balance,
       COALESCE(SUM(CASE le.direction WHEN 'credit' THEN le.amount ELSE -le.amount END), 0)::bigint AS ledger_balance
FROM wallets w
LEFT JOIN ledger_entries le ON le.wallet_id = w.id
GROUP BY w.id, w.balance
HAVING w.balance <> COALESCE(SUM(CASE le.direction WHEN 'credit' THEN le.amount ELSE -le.amount END), 0);

-- name: InsertBankStatement :one
-- Every callback books a statement row, duplicates included — recon flags
-- the second row for the same ref_code (E15).
INSERT INTO bank_statements (ref_code, amount, stmt_date, raw)
VALUES ($1, $2, $3, $4)
RETURNING id;
