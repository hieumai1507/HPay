---
id: BUG-0001
title: A redelivered MOVE_FUNDS step posts the same transaction to the ledger twice
status: fixed
severity: blocker
area: backend/ledger
tags: [replay, outbox, workflow, state-machine]
audit: [BE-C1]
found: 2026-11-21
fixed: 2026-11-21
commit: pending (fix is in the working tree, not committed)
regression_test: src/backend/internal/ledger/posting_guard_test.go:TestPost_SecondCallSameKindIsRejected ; src/backend/test/integration/money/ledger_replay_test.go:TestEdge_E06_DuplicateStepNoDoublePost
related: []
---

# BUG-0001: A redelivered MOVE_FUNDS step posts the same transaction to the ledger twice

## Symptom
Posting the same transaction through `ledger.Post` twice moved the money twice: in the reproduction the sender balance went 900 -> 800 after a second post of one 100-unit transfer. In production the trigger is a worker crash or an outbox redelivery between the handler's database commit and the recording of the step: the handler runs again and debits the sender and credits the receiver a second time. Compensation had the same exposure.

## Root cause
`workflow/handlers/handlers.go` (MOVE_FUNDS) commits `ledger.Post` in its own database transaction, and only afterwards does the worker record `step_executions` and the outbox event in a second transaction (`workflow/consumer.go` `recordAndEmit`). Delivery is at-least-once. `ledger.Post` never checked whether the transaction had been posted: `ledger_entries` had only a non-unique index on `transaction_id`. A unique key on `transaction_id` alone was not possible because compensation and round-up legitimately post again on an already-posted transaction.

## Other places with the same flaw
All eight call sites of `ledger.Post` / `PostWithQueries` were reviewed with GitNexus impact analysis (risk HIGH, 4 execution flows): MOVE_FUNDS forward and compensating, round-up, user top-up (`wallet/service.go`), bank callback top-up (`wallet/callback.go`), org top-up (`org/service.go`), reversal executor (`reversal/reversal.go`), and the demo seeder. Only the workflow handlers are exposed to redelivery; the others run inside one request transaction that also changes the transaction's own state, but all of them now pass through the same guard.

## Fix
ADR-0010. New table `ledger_postings` with PRIMARY KEY (transaction_id, kind), kind in forward / compensate / round_up (migration 000014, back-filled for existing postings). `ledger.Post(ctx, tx, txnID, kind, legs)` claims the row after the ordered wallet locks and before writing entries. A second claim with the same total returns `ErrAlreadyPosted` and changes nothing; a different total returns `ErrPostingConflict`. MOVE_FUNDS and round-up treat `ErrAlreadyPosted` as success (`replayed: true`) and `ErrPostingConflict` as terminal.

## Regression test
- `TestBEC1_SecondPostOfSameTransactionMovesMoneyTwice` first failed against the old code (900 -> 800), then passes.
- `TestPost_*` (7 cases: replay, conflict, concurrency with 8 goroutines, compensate once, round-up once, failure leaves no guard) and `TestMigration_BackfillCoversEveryPostedTransaction`, `TestLedgerEntriesStayAppendOnly`.
- `TestEdge_E06_DuplicateStepNoDoublePost` runs the registered MOVE_FUNDS handler a second time after a completed transfer. Verified to fail with the guard disabled (error `insufficient balance` instead of an idempotent success) and to pass with it restored.
- Full suite after the change: 170 tests, 0 failures, with `-race`.

## How to recognize this again
- A wallet balance that differs from the sum of its ledger entries, or a transaction with more ledger entries than legs.
- Two identical sets of `ledger_entries` rows for one `transaction_id` and no matching second `step_executions` attempt.
- Logs showing a MOVE_FUNDS step executing twice for one instance after a restart or a Redis hiccup.
- Recon reports drift for a wallet that took part in one transaction only.

## Docs updated
`domain/ledger.md` (BR-LED-15, BR-LED-16, known gaps), `domain/workflow-engine.md`, `reference/data-model.md`, `architecture/crosscutting-concepts.md`, `constitution.md` (C-02, C-06 status), `adr/0010-ledger-postings-guard.md`.