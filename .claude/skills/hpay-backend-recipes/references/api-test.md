# Generate integration tests for endpoints

> Trigger: Generate integration tests for hpay-backend endpoints against a real PostgreSQL + Redis (docker compose). Trigger on "write tests for endpoint X", "test this flow", or when an edge case ID (E01–E20, M, A) needs coverage.

# gen-api-test — Integration tests with real infra

## Setup pattern (reuse, don't reinvent)

- Shared harness in `internal/testutil/`: `PoolFromEnv` connects to `TEST_DATABASE_URL` (DB must already be migrated: `docker compose up -d && make migrate-up`) and SKIPS the test when unset. The repo does NOT use testcontainers (not in go.mod). Router + worker run in-process (`--role=all`).
- All tests share ONE database (no per-test schema). Tests must not depend on order or on an empty database: create their own users/wallets and assert only on rows they created; clean up what they must in `t.Cleanup`. `go test ./...` runs packages in parallel against that same database.
- Helpers that exist today (use these; do not invent others), all in `test/testkit` (package `testkit`, import `github.com/hieumai1507/hpay-backend/test/testkit`): `testkit.NewEnv(t, defName)` (router + engine + worker in-process, returns `*testkit.Env`), `env.Drain(t)` (pump the outbox for this env), `env.UserWithMoney(t, amount)` (returns user id, wallet id, phone), `env.SeedAdminID(t)` / `SeedAdmin2ID(t)`, `env.SetScore(n)` (KYC mock scorer), `env.TxnLedgerNet(t, refCode, walletID)`, `testkit.PostTransfer(...)`, `testkit.Fund(...)`, `testkit.ErrIs(...)`, `testkit.NewAdminRig(t)` (full router with admin login), `testkit.NewOrgFixture(...)`, `env.SetPIN`, `env.RequestP2P`, `env.ConfirmWithPIN`, `env.SetOffer`. `internal/testutil` only provides `PoolFromEnv` and the shared `TestMain` setup. If a needed helper is missing, add it to the matching file in `test/testkit` once and reuse it.
- The engine is async: after a request, call `env.Drain(t)` and then poll the transaction status with a deadline; never `time.Sleep` for correctness.
- Redis-backed legs (E05, E17) need `TEST_REDIS_ADDR` set; without it those legs are skipped while the test still reports PASS. `scripts/test-backend.ps1` does not set it today, so do not count those two as proven by that runner.

## What every endpoint test suite must include

1. **Happy path** — assert response shape AND database effects (ledger rows, balances, statuses), not just HTTP 200.
2. **Idempotent retry** — same Idempotency-Key twice → identical response, single DB effect.
3. **AuthZ** — wrong user/role/org gets 403/404; verify another user's resource is invisible.
4. **Validation** — amount 0, negative, over KYC limit, self-transfer → correct `ERR_*` code.
5. **Async settlement** — for engine flows, wait for terminal status and assert the full `step_executions` trail.

## Edge-case IDs

When the user names an ID from docs (E01–E20 backend, M mobile-contract, A admin), name the test `TestEdge_<ID>_<slug>` — CI's `check-edge-tests.sh` greps for these. Key recipes:

- **E02 (double spend):** two goroutines transfer from same funded wallet, total > balance; assert exactly one `completed`, one `failed ERR_INSUFFICIENT_BALANCE`, balance never negative (query, don't trust response).
- **E03 (cross transfer):** A→B and B→A concurrently; assert both complete within timeout (no deadlock) and ledger sums match.
- **E06 (duplicate event):** publish the same step event twice via test hook; assert single ledger effect.
- **E08 (checker race):** two checkers approve concurrently on a 1-signature tier; one gets `ERR_ALREADY_FINALIZED`.
- **E09 (policy snapshot):** create pending approval → update policy → assert evaluation still uses snapshot.

## Rules

- Always run with `-race`, through `.\scripts\test-backend.ps1 -GoArgs "./test/integration/... -race -count=1 -v"` on this machine (CI runs `go test -p 1`; the local script does not, so a test that only fails in parallel needs `-p 1` to reproduce or to rule out).
- No `time.Sleep` for correctness — use polling helpers with deadline.
- Never mock Postgres/Redis in integration tests; mock only external AI/bank clients (use the built-in mock scorer).

## Test placement rules (user rule — follow exactly)

- **Unit tests & white-box tests**: colocated `*_test.go` NEXT TO the code, same package. This is the Go convention and is compiler-enforced for white-box tests (access to unexported identifiers). Examples: `internal/auth/otp_test.go`, `internal/workflow/integration_test.go` (white-box engine tests).
- **Cross-module black-box integration/E2E tests**: ALL go in `test/integration/<area>/`, one Go package per area: `money` (transfer, wallet, vault, idempotency, offer limits), `authn` (PIN gate, login), `kyc`, `org` (organizations and maker-checker approvals), `admin` (admin API, RBAC, reversals, recon, audit, workflows). Each package has a `main_test.go` with `TestMain` calling `testutil.RunSerialized`. Shared harness lives in `test/testkit` (env, drain via Worker.PumpLocal, UserWithMoney, PostTransfer, TxnLedgerNet). Never duplicate these helpers; a helper used by two areas goes in `test/testkit`, one used by a single area stays in that package. Run one area with `go test ./test/integration/money/...`.
- New edge-case tests (TestEdge_<ID>_...): if the scenario spans modules (API → engine → ledger), it belongs in `test/integration/`; if it tests one package's pure logic, colocate it.

## Shared-database parallelism (lesson learned — do not repeat)

`go test ./...` runs DIFFERENT packages in PARALLEL against the SAME test
database. Any harness that drains the global outbox (or any shared queue
table) MUST only consume rows it owns:
- `Worker.PumpLocal` skips start-events whose handler is not in its registry
  (leaves them unpublished for the owning package's pump).
- Test mini-relays must filter rows by their own instance/transaction id.
Never write a test helper that claims + marks ALL shared rows — it will steal
another package's events and fail only in CI (parallel) while passing locally.
