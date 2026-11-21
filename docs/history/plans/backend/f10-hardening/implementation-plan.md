[← F9](../f9-fraud-recon-reversal/implementation-plan.md) · f10-hardening · [solution-architecture →](solution-architecture.md)

# F10 Hardening — Implementation Plan

## 1. Scope & non-goals

> **F10 adds no new business features** — it closes out the deliberate debts left over from F1–F9 to reach the Definition of Done for the whole backend (master plan §4, week 5): **(a)** full coverage of edge tests E01–E20 (E15 and E17 are missing — F9 explicitly stated in its non-goals "full E15 test in F10"), **(b)** a k6 load test at 100 TPS transfer + results in the README (NFR doc 01 §5), **(c)** security scanning with govulncheck + trivy in CI, **(d)** a complete `cmd/seed` following the `seed-demo-data` skill — the foundation for the demo and for the M/A phases. The only business part is the **mock bank callback endpoint** — required so that E15 can be tested with the exact semantics "callback arrives twice" (topup is currently a synchronous mock with no callback path).

| Ships | Not in F10 |
| --- | --- |
| Mock bank callback endpoint `POST /internal/mock-bank/callback` (idempotent by `ref_code`, the 2nd call writes a duplicate `bank_statements` row for recon) + `TestEdge_E15`; `TestEdge_E17` Redis down (API still writes DB + outbox, healthz 503 degraded, Redis back up → relay resumes on its own); `check-edge-tests.sh` REQUIRED covering E01–E20; k6 script `scripts/loadtest/k6-transfer.js` + Makefile target + results in README; CI jobs `govulncheck` + `trivy` (image scan on the existing Dockerfile); idempotent `cmd/seed/main.go` covering all 7 items of the seed-demo-data skill + Makefile `seed`; fix fatal findings if the scans detect any | Real bank integration / real webhook signature verification (a mock signed with a shared secret env is enough for a portfolio); deep performance tuning if k6 meets the target; account_lock executor (deferred to the A-phases as F9 noted); notification; business schema changes; chaos tests other than E17 |

## 2. Task breakdown

| # | Task | Files | Size | Depends |
| --- | --- | --- | --- | --- |
| T1 | **Mock bank callback:** `POST /internal/mock-bank/callback` — body {ref_code, amount, bank_ref}; guarded by header `X-Mock-Bank-Secret` (env `MOCK_BANK_SECRET`, dev default). 1st call: if the topup is in status `created` (the "waiting for bank" state — use the existing enum, do NOT add an enum value; add an internal function `CreatePendingTopup` that creates a `created` topup without posting to the ledger, used by test/seed) → complete via `ledger.Post`, transition guarded `WHERE status='created'`. The current synchronous topup stays unchanged — a callback for a topup already `completed` goes straight to the duplicate branch; INSERT a `bank_statements` row (feeds recon). 2nd call with the same `ref_code`: no money change, INSERT a 2nd statement → recon will see the duplicate (matches F9's E15/E16 design). New queries in `topup.sql`; register in router + errmap | internal/wallet/callback.go, internal/store/queries/topup.sql, internal/api/router.go, api/openapi.yaml | M | — |
| T2 | **TestEdge_E15_DuplicateBankCallback** (integration): call the callback twice with the same ref_code → balance is credited exactly once, the ledger has exactly 1 pair of entries, bank_statements has 2 rows, recon RunForDate marks it as duplicate | test/integration/wallet_test.go | M | T1 |
| T3 | **TestEdge_E17_RedisDownNoDataLoss** (integration): point the engine to a dead Redis addr → POST transfer still returns 202, the transaction + workflow_instance + outbox row exist in the DB; healthz returns 503 degraded; point back to the real Redis → relay + consumer run, the transfer completes. Use the existing helpers_test.go, do not duplicate it | test/integration/redis_down_test.go, test/integration/helpers_test.go | M | — |
| T4 | **check-edge-tests.sh:** REQUIRED by default = full E01–E20 (E18 is still skipped because the CI migrate job covers it); update `ci.yml` REQUIRED_EDGE_IDS to all 20 IDs | scripts/check-edge-tests.sh, .github/workflows/ci.yml | S | T2,T3 |
| T5 | **k6:** `scripts/loadtest/k6-transfer.js` — constant-arrival-rate scenario 100 TPS × 60s: log in a pool of pre-seeded users → POST transfer (unique Idempotency-Key) → poll detail until completed; thresholds p95 read <100ms, transfer e2e <2s, error rate <1%. Makefile `loadtest`. Run on your machine, paste the results into README §Load test | scripts/loadtest/k6-transfer.js, Makefile, README.md | M | T7 |
| T6 | **Security CI:** job `vuln`: `govulncheck ./...`; job `image-scan`: build Dockerfile + `trivy image --exit-code 1 --severity HIGH,CRITICAL` (aquasecurity/trivy-action). Fix fatal findings if any (bump deps) | .github/workflows/ci.yml | S | — |
| T7 | **cmd/seed:** idempotent via fixed UUIDs, covering all 7 items of the seed-demo-data skill (users 0900000001–04, org DEH Corp + policy v3 + disbursement 1/2 signatures, workflow variety failed/compensated/processing_async, fraud flags, yesterday's recon with 3 missing_external in the 23:58–00:02 window, money request + split group); every balance = SUM of the ledger per wallet, assert at the end, fail loudly; timestamps relative to now(); print a summary table. Makefile `seed` | cmd/seed/main.go, Makefile | L | — |
| T8 | **Docs sync:** update README (edge coverage 20/20, load test, security scan, seed demo guide); update plan files if they drift | README.md, docs/plans/f10-hardening/* | S | T5 |

## 3. Schema changes

**No new migration.** E15 uses the `transactions` table (idempotent by `ref_code`, already UNIQUE) + `bank_statements` (existing since F9, allowing multiple rows with the same ref so recon catches duplicates — re-confirm the constraint when coding; if `bank_statements.ref_code` is currently UNIQUE then a migration is needed to relax it to non-unique + a plain index, gen-migration will be run and this plan updated).

## 4. API changes

| Endpoint | Change | Errors |
| --- | --- | --- |
| `POST /internal/mock-bank/callback` | New — mock bank notifies a topup; guarded by `X-Mock-Bank-Secret`; response {ref_code, status, duplicate:bool} | `ERR_INVALID_CALLBACK_SECRET` 401 · `ERR_TXN_NOT_FOUND` 404 · `ERR_CALLBACK_AMOUNT_MISMATCH` 422 |

openapi.yaml is updated in the same commit as T1. No other endpoint changes.

## 5. Test plan

| ID | Test | Location |
| --- | --- | --- |
| E15 | `TestEdge_E15_DuplicateBankCallback` — credit once, 2 statements, recon sees duplicate | test/integration/wallet_test.go |
| E17 | `TestEdge_E17_RedisDownNoDataLoss` — Redis down: DB+outbox still written, healthz degraded, recovery resumes on its own | test/integration/redis_down_test.go |
| — | Unit: wrong callback secret/amount mismatch; seed assert balance=SUM(ledger) is itself the seeder's self-test | internal/wallet/callback_test.go |
| E01–E14, E16, E19, E20 | Already exist from F1–F9 — check-edge-tests.sh now enforces the full list in CI | — |

## 6. Rollout & risk

- **Callback endpoint under `/internal/`** — not in the user route group; if deployed for real it must be blocked at the reverse proxy. Secret via env, not hardcoded.

- **k6 results depend on the machine** — thresholds are set per the NFR; if the laptop does not reach 100 TPS, record the real numbers in the README with a bottleneck analysis, do not change the thresholds to get "green".

- **trivy may fail because of an old base image** — handled by bumping the image/deps, which is the purpose of this phase.

- **Seeder runs on the dev DB** — idempotent via fixed UUIDs so re-running is safe; never run it on a DB with real data (guard: refuse to run if `ENV=production`).

- A partial merge breaks nothing: every task is independent of the existing runtime except T1 (a new endpoint, additive).

## 7. Execution checklist

1. gen-feature-plan ✔ (this document) → **awaiting approval**

2. T1 code callback (+ gen-migration **only if** bank_statements needs UNIQUE relaxed) → sqlc generate clean → build

3. T2, T3 tests (gen-api-test conventions, map E15/E17) → T4 script

4. T6 CI security → T7 seed → T5 k6 (needs seeded users first)

5. fintech-review → VERDICT READY → you run verify → commit

---

F10 Hardening · implementation plan v1 · 18/07/2026
