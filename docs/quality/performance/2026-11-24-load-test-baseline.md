---
title: P2P load test baseline and the improvements it points to
type: quality
status: measured
last_verified: 2026-11-24
related:
  - planning/PLAN.md
---

# P2P load test baseline (2026-11-24)

Measured by the owner on a fresh clone of `main`. The test is `src/backend/test/loadtest/p2p_load_test.go` (build tag `loadtest`, not part of the normal suite).

## Setup
Docker on Linux, Postgres 16, Redis 7, `golang:1.26`, load container limited to 4 CPUs, `TEST_DB_MAX_CONNS=40`, sqlc 1.27.0. The test drives the real service path (`ExecuteEngineRequest`, then `ExecuteEngineConfirm` with the PIN, then the workflow through `outbox.NewRelay`, Redis Streams and `workflow.NewWorker(...).Run`) and checks integrity at the end.

Run (fresh migrated database):
```
cd src/backend
LOAD_USERS=200 LOAD_WORKERS=50 LOAD_SECONDS=60 \
TEST_DATABASE_URL=postgres://HPay:HPay@localhost:5432/load?sslmode=disable \
REDIS_ADDR=localhost:6379 APP_ENV=ci \
go test -tags loadtest ./test/loadtest -run TestP2PLoad -v -count=1 -timeout 15m
```

## Results
| Metric | Value |
|---|---|
| Transfers accepted (request + PIN confirm) | 600 ok, 0 failed, about 9.6 req/s |
| Request + confirm latency | p50 5.0 s, p95 7.0 s, p99 7.7 s |
| Transfers completed | 600 of 600, 0 failed, about 7.9 completed/s |
| End to end (created to completed) | p50 42.6 s, p95 67.1 s, p99 69.4 s (queueing) |
| Integrity | 0 wallets with balance different from the ledger sum; money among users before minus after equals the fees exactly |
| `go test ./... -race` in parallel | 9 failures in `test/integration/*` (shared database: deadlock 40P01 and stolen outbox events); all pass with `-p 1`; 0 data races |

Conclusion: correctness under concurrency holds; throughput is low (about 8 transfers/s).

## Findings and status
| ID | Finding | Where | Status |
|---|---|---|---|
| P1 | Each workflow stream has one consumer goroutine; about 13 outbox events per transfer cap completion near 8/s | `internal/workflow/consumer.go` (`Worker.Run`, `consumeLoop`, `Count: 10`) | done in code (G23), see "After" |
| P2 | PIN check uses login-grade argon2id (64 MiB, t=3) on every payment: 50 concurrent confirms need about 3.2 GB and saturate the CPUs | `internal/auth/password.go`, `VerifyPIN` | semaphore done (G25); lighter PIN parameters still open, needs an ADR |
| P3 | Outbox relay polls every 200 ms with a batch of 100 (ceiling 500 events/s) | `internal/outbox/relay.go` | done in code (G24) |
| P4 | A fresh clone does not build: `internal/store/gen` is gitignored and the quick start does not stress that `make sqlc` is required | README, `scripts/test-backend.*` | done |
| P5 | The k6 script calls routes that no longer exist (`/v1/transactions/transfer`, `/v1/wallets/topup`) and ignores the login and confirm rate limits | `scripts/loadtest/k6-transfer.js`, `make loadtest` | done |
| P6 | Integration packages are not safe to run in parallel against one database; `PumpLocal` marks events published before processing them | CI and `scripts/test-backend.sh`, `workflow/worker.go` | PumpLocal fixed; funding lock order not yet checked |
| P7 | Items already in PLAN.md: G7 (login lockout: done by BUG-0016), G18 (limit windows race), G19 (outbox idempotency for non-money steps), G1 (done: Go 1.26), G3 (gosec and gitleaks not in CI) | `docs/planning/PLAN.md` | partly done |

## Order chosen by the owner (2026-11-24)
P4, P5, P6 first (low risk), then P1, P3, P2. Re-run this load test after P1, P3 and P2 with the same settings and record the new numbers in this folder. P2 is a security trade-off and gets an ADR before any change to hashing parameters.

## Not verified
These numbers come from the owner's run. They have not been reproduced by the assistant, and the machine, container limits and database state change them.
## After P1, P3 and P2 (assistant's run, same settings)
Settings: 200 users, 50 workers, 60 s, fresh migrated database `load`, `TEST_DB_MAX_CONNS=40`, Docker Desktop on the dev laptop (not the owner's machine, so compare the shape, not the absolute numbers). Changes: 8 consumers per stream (`WORKFLOW_CONSUMERS`), relay loops while batches are full, argon2 concurrency bounded to the CPU count.

| Metric | Before (owner) | After |
|---|---|---|
| Accepted (request + confirm) | 600 ok, 0 failed, 9.6 req/s | 632 ok, 0 failed, 9.9 req/s |
| Request + confirm latency p50 / p95 | 5.0 s / 7.0 s | 4.9 s / 6.3 s |
| Completed | 600, 0 failed, 7.9/s | 628 completed, 4 failed (fraud rules, expected under 200 users hammering each other), 9.1/s |
| End to end p50 / p95 / p99 | 42.6 / 67.1 / 69.4 s | 20.6 / 34.6 / 37.1 s |
| Ledger mismatches | 0 | 0 |

Reading: the workflow no longer queues behind a single consumer (end-to-end latency about halved, the backlog drains right after intake stops), but intake is still about 10 requests per second. Intake is bound by the argon2 PIN check on the CPUs; the semaphore bounds memory and keeps latency stable, it cannot raise CPU-bound throughput. Raising intake needs cheaper PIN hashing (OWASP minimum m=19 MiB, t=2, p=1) or more CPUs; that is a security trade-off for the owner and needs an ADR. Not measured: a run on the owner's machine, and a run with lighter PIN parameters.
