---
name: fintech-review
description: Review pending changes in hpay-backend against fintech invariants before commit or PR. Trigger on "review", "check my code", "before I commit", or automatically as the final step of any feature work in this repo.
---

# fintech-review — Invariant checklist gate

Run every check below against the working diff (`git diff` + `git diff --staged`) AND the files it touches. Report a table: check | PASS/FAIL | file:line evidence. Any FAIL blocks commit — propose the fix.

## Checks

1. **No float money.** Grep changed files for `float32|float64` used with amount/balance/fee/total. Money is `int64` minor units end-to-end (DB `BIGINT`, JSON as integer, client BigInt).
2. **Idempotency.** Every new/modified POST endpoint that creates a transaction, approval action, or money request reads `Idempotency-Key` via the middleware and relies on the DB unique constraint. No handler bypasses it.
3. **Lock ordering.** Every `SELECT ... FOR UPDATE` on wallets sorts ids ascending before locking. Never lock inside a loop.
4. **Ledger immutability.** No `UPDATE ledger_entries` / `DELETE FROM ledger_entries` anywhere (migrations included). Reversals INSERT opposite entries with `reversal_of` linkage.
5. **State machine discipline.** No direct `UPDATE ... SET status = ...` on transactions/workflow_instances/approvals outside the owning transition function. Grep for raw status writes.
6. **Outbox only.** No direct `XADD`/publish from business code — events enqueue via outbox inside the same DB tx as the state change.
7. **Secrets & PII in logs.** No logging of password, OTP code, token, full phone, balance at info level or above. Check slog calls in the diff. OTP stored hashed only.
8. **AuthZ two layers.** New queries touching user-owned rows include owner predicate in SQL, not just middleware. Admin endpoints have role check + audit log write with actor + reason where destructive.
9. **Error contract.** New failure paths return `ERR_*` codes registered in `errmap.go` and documented in `openapi.yaml`. No raw error strings to clients.
10. **Tests & race.** `go vet ./...`, `golangci-lint run`, `go test ./... -race` all pass. If the diff claims to cover an edge case ID (E/M/A), the corresponding `TestEdge_<ID>_*` exists.

## Output format

End with one of:
- `VERDICT: READY TO COMMIT` — all pass.
- `VERDICT: BLOCKED (n fails)` — list fixes in priority order, offer to apply them.
