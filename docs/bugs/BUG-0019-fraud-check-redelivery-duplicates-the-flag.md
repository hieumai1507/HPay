---
title: A redelivered FRAUD_CHECK step writes a second fraud flag (and a second advisory call)
id: BUG-0019
status: fixed
severity: medium
area: backend/fraud
tags: [idempotency, replay, outbox]
audit: [BE-C1]
found: 2026-11-24
fixed: 2026-11-24
commit: pending
regression_test: src/backend/test/integration/money/handler_redelivery_test.go:TestHandlerRedelivery_RunningTwiceAddsNoSideEffects/fraud ; TestHandlerRedelivery_FraudRunWaitsForTheRunHoldingTheTransactionLock
related: [BUG-0001, BUG-0020]
---
# BUG-0019: redelivered fraud check duplicates the flag
## Symptom
Running the `fraud` handler twice for the same instance, step and attempt left two `fraud_flags` rows for one transaction (grey zone score 50: flags=1 after run 1, flags=2 after run 2). The review queue showed the same transaction twice and the LLM advisory was paid for twice.
## Root cause
`handler.Execute` did a plain `INSERT INTO fraud_flags` before `recordAndEmit` recorded the step. A crash, timeout or XAUTOCLAIM between the two makes the broker redeliver the start message and the handler runs again. `fraud_flags` has no unique key on `transaction_id`, and migrations are frozen for this change.
## Fix
`insertFlagOnce` (`fraud/handler.go`): in one transaction take `pg_advisory_xact_lock` on the transaction id, read the existing flag (`GetFraudFlagIDByTransaction`), insert only if there is none. A redelivery reuses the flag and does not start a second advisory. The lock serialises two runs that overlap (a slow original plus an XAUTOCLAIM copy). A unique index on `fraud_flags(transaction_id)` would be the stronger guard; it needs a migration and existing duplicate rows may have to be merged first (assumption: not done here, see Not done).
## Regression test
Table test runs every registered handler twice and compares side-effect snapshots; `fraud` was red (flags=2). `TestHandlerRedelivery_FraudRunWaitsForTheRunHoldingTheTransactionLock` holds the lock itself and requires the handler to wait. Mutation check: replacing the lock statement with a no-op made the second test fail ("it finished"); removing the existence check makes the table test fail (observed red before the fix).
## How to recognize this again
Two `fraud_flags` rows with the same `transaction_id`.
## Docs updated
`domain/outbox-and-events.md`, `domain/workflow-engine.md`, `domain/fraud-and-recon.md`, `planning/PLAN.md` (G19).
## Not done
- No unique index on `fraud_flags(transaction_id)`; existing duplicates from before the fix are not cleaned.