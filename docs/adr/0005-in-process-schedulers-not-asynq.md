---
title: ADR 0005 In-process schedulers, not asynq
type: decision
status: verified-against-code
last_verified: 2026-11-21
---

# ADR 0005: In-process schedulers, not asynq

Status: Accepted (verified against `cmd/server/main.go` and `go.mod` 2026-11-21)

## Context
Older design documents (00, 01, 04 and the index) said cron and recovery jobs run on asynq, migrations use the golang-migrate library, and tests use testcontainers. The audit appendix confirmed that none of these libraries is in `go.mod`; the documents were wrong.

## Decision
Background work runs as goroutines started in `cmd/server/main.go` when the role includes workers (`worker` or `all`):
- `outbox.Relay.Run` (200 ms poll)
- `workflow.Worker.Run` (stream consumers)
- `workflow.Recovery.Run` (every 30 s: `XAUTOCLAIM` plus the stuck-instance sweep with an `UPDATE ... SKIP LOCKED` lease)
- `approval.Sweeper.Run` (every 30 s, expires overdue approvals)
- `moneyrequest.Sweeper.Run` (expires unpaid money requests)
- `recon.Cron.Run` (nightly reconciliation at 00:15)

Migrations are applied by the external `migrate` CLI (Makefile, CI, `scripts/test-backend.ps1`), never by the server. Integration tests use docker compose or CI service containers, not testcontainers.

## Consequences
- Fewer moving parts and dependencies.
- Not horizontally safe by default: the recovery sweep and outbox claims use row leases and are safe with several replicas, but the recon cron has no lease and the sweepers are not coordinated, so run exactly one worker process until that is fixed (audit, confirmed).
- No persistence of schedule or retry state outside Postgres and Redis streams; a process restart simply resumes from the tables.
- Documents that still mention asynq (history) are wrong and must not be used.