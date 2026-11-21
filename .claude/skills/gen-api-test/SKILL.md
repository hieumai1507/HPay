---
name: gen-api-test
description: Generate integration tests for hpay-backend endpoints using testcontainers (real PostgreSQL + Redis). Trigger on "write tests for endpoint X", "test this flow", or when an edge case ID (E01–E20, M, A) needs coverage.
---

# gen-api-test — Integration tests with real infra

## Setup pattern (reuse, don't reinvent)

- Shared harness in `internal/testutil/`: spins up **testcontainers-go** Postgres 16 + Redis 7 once per package (`TestMain`), runs all migrations, returns a ready `*App` with router + worker running in-process (`--role=all` mode).
- Each test gets an isolated DB schema OR truncates tables in `t.Cleanup` — tests must not depend on order.
- Helpers: `testutil.NewUser(t, kycTier)`, `testutil.Fund(t, walletID, amount)`, `testutil.Do(t, method, path, body, opts...)` (auto JWT + Idempotency-Key), `testutil.WaitTxnStatus(t, refCode, status, 5*time.Second)` (poll — the engine is async).

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

- Always run with `-race`.
- No `time.Sleep` for correctness — use polling helpers with deadline.
- Never mock Postgres/Redis in integration tests; mock only external AI/bank clients (use the built-in mock scorer).

## Test placement rules (user rule — follow exactly)

- **Unit tests & white-box tests**: colocated `*_test.go` NEXT TO the code, same package. This is the Go convention and is compiler-enforced for white-box tests (access to unexported identifiers). Examples: `internal/auth/otp_test.go`, `internal/workflow/integration_test.go` (white-box engine tests).
- **Cross-module black-box integration/E2E tests**: ALL go in `test/integration/` (package `integration`). Shared harness lives in `test/integration/helpers_test.go` (env, drain via Worker.PumpLocal, userWithMoney, postTransfer, txnLedgerNet). Never duplicate these helpers.
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
