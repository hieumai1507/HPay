---
title: A redelivered CONFIRM step fails because the transaction is already completed
id: BUG-0020
status: fixed
severity: high
area: backend/workflow
tags: [idempotency, replay, state-machine]
audit: [BE-C1]
found: 2026-11-24
fixed: 2026-11-24
commit: pending
regression_test: src/backend/test/integration/money/handler_redelivery_test.go:TestHandlerRedelivery_RunningTwiceAddsNoSideEffects/confirmer
related: [BUG-0001, BUG-0019]
---
# BUG-0020: CONFIRM is not replay-safe
## Symptom
Running the `confirmer` handler a second time for the same instance, step and attempt returned `no rows in result set` as a retryable error. In production this is the path "CONFIRM committed, the worker crashed before recording the step": the redelivered run keeps failing, the retries run out and `failInstance` fails the instance while the transaction is already `completed` (money moved, status contradictory).
## Root cause
`CompleteTransaction` is a guarded UPDATE (`WHERE status='processing' ... RETURNING id`). On a transaction that is already completed it returns no row, and the handler mapped every error, including `pgx.ErrNoRows`, to `RetryableError`.
## Fix
`Confirmer.Execute` treats `ErrNoRows` as success when the transaction is already `completed` (returns `confirmed: true, replayed: true`). Any other status (for example `failed`) still returns the retryable error as before.
## Regression test
Table test `confirmer` case (red: "no rows in result set"). Mutation check: reverting the `ErrNoRows` branch makes it fail again.
## How to recognize this again
Instance `failed` after CONFIRM with error text `no rows in result set` while the transaction row is `completed`.
## Docs updated
`domain/outbox-and-events.md`, `domain/workflow-engine.md`, `planning/PLAN.md` (G19).
## Not done
- The confirm notification ("F-later: notification push") does not exist yet; when added it needs its own idempotency key.