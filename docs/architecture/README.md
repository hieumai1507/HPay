---
title: Architecture - index and system summary
type: architecture
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/cmd/server/main.go, src/backend/internal/api/router.go, src/backend/go.mod]
---

# HPay architecture (as built today)

## System summary

1. HPay is a US-dollar (USD) e-wallet: P2P transfer, money requests / split bill, vaults (savings pockets), bank top-up, KYC, and business (org) wallets with maker-checker disbursement.
2. One Go binary (`src/backend/cmd/server`, Gin + pgx + go-redis) serves everything; `--role=api|worker|admin|all` chooses which HTTP surfaces and background loops run (`main.go:36`, `config.go:13-41`).
3. State lives in PostgreSQL (append-only double-entry ledger, workflow tables, outbox); Redis carries the workflow streams (`wf:steps:<handler>`, `wf:events`) and rate-limit counters.
4. Money-moving transactions are executed by a DB-defined workflow engine (steps VALIDATE, FRAUD_CHECK, ..., MOVE_FUNDS, CONFIRM) driven by a transactional outbox, not inline in the HTTP request.
5. Two clients: `src/mobile` (Expo / React Native, end users and business users) and `src/admin` (React + Vite back-office). Background jobs (outbox relay, recovery, sweepers, nightly recon) are in-process goroutines; migrations are applied by the external `migrate` CLI.

## Sections

| File | Content |
|---|---|
| [context-and-scope.md](context-and-scope.md) | Actors, external systems, scope boundary |
| [building-blocks.md](building-blocks.md) | Repo layout, backend packages by layer, process roles |
| [runtime-views.md](runtime-views.md) | Five key scenarios as sequence diagrams |
| [deployment.md](deployment.md) | Docker, compose, CI, config, migrations, dev machine |
| [crosscutting-concepts.md](crosscutting-concepts.md) | Money, idempotency, locking, outbox, errors, auth, logging, rate limit, audit, timezone |
| [design-principles.md](design-principles.md) | SOLID, design patterns and code smells for HPay (guidance for implementers and reviewers) |
| [admin-app.md](admin-app.md) | `src/admin` in detail |
| [mobile-app.md](mobile-app.md) | `src/mobile` in detail |

Product-side docs: [../product/feature-catalog.md](../product/feature-catalog.md), [../product/roles-and-permissions.md](../product/roles-and-permissions.md), [../product/kyc-tiers-and-limits.md](../product/kyc-tiers-and-limits.md).

## Reading rules

- Every statement here was checked against code on 2026-11-21 (code was read, not executed; no test was run for this documentation pass).
- Known defects are tracked in [../audit/AUDIT-REPORT.md](../audit/AUDIT-REPORT.md); this section cites audit IDs (BE-C1, BE-H1, ...) next to the code that has the problem and does not repeat the whole report.
- Old design docs live in `docs/history/` and are context only. They contain claims the code does not support (asynq, golang-migrate, testcontainers in go.mod; `HPay serve` subcommand; features of other products). Where this matters it is called out explicitly.
