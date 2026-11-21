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
11. **Idempotency scope.** The key is unique per user (`initiated_by`, key) and bound to a request hash; same key + different payload returns 409. No server-side fallback that generates a random key when the client sent none.
12. **No double posting.** Any ledger post is guarded by a DB uniqueness rule on (transaction, posting kind); a replayed workflow step or redelivered outbox event cannot post twice. Side effects and the step record share one transaction or a claim row.
13. **Authentication is real.** PIN/OTP is verified server-side (hash, attempt counter, lockout); fields accepted but unused are a FAIL. Transaction reads check `initiated_by`.
14. **Admin RBAC.** Every admin route checks the specific role/permission, not just "is admin"; destructive or config actions write an audit row with actor and reason.
15. **Environment guards.** No default secret accepted outside `dev`; dev-only behavior (otp_dev, mock gateway, mock-bank route) is gated on an explicit validated `APP_ENV`.
16. **Outbound URLs.** Any URL from config/DB passes scheme+host allowlist and private-IP block before request.
17. **Frontend money.** No `Number()`/`parseInt()`/float math on amounts; writes send an idempotency key and are disabled while pending; no PIN or secret literals.
18. **Timezone.** Day/month limits use a fixed zone (Asia/Ho_Chi_Minh), not server local time.
## Output format

End with one of:
- `VERDICT: READY TO COMMIT` — all pass.
- `VERDICT: BLOCKED (n fails)` — list fixes in priority order, offer to apply them.
