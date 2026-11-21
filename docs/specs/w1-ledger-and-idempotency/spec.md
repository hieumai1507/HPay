---
title: Wave 1 slice 1 - a ledger posting can happen at most once
type: spec
status: proposed
last_verified: 2026-11-21
audit: [BE-C1]
related:
  - ../../domain/ledger.md
  - ../../domain/workflow-engine.md
  - ../../adr/0010-ledger-postings-guard.md
---

# Wave 1 slice 1: a ledger posting can happen at most once (BE-C1)

## Problem
`MOVE_FUNDS` commits `ledger.Post` in its own transaction, and only afterwards does the worker record `step_executions` and the outbox event in a second transaction (`consumer.go` `recordAndEmit`). If the process dies between the two, or the event is redelivered (the outbox is at-least-once), the handler runs again and posts the same legs a second time. Nothing in the schema stops it: `ledger_entries` has only a non-unique index on `transaction_id`, and `Post` never checks whether the transaction was already posted. Result: sender debited twice, receiver credited twice. The same applies to compensation.
Evidence: `docs/domain/ledger.md` (BR-LED-*), `docs/domain/workflow-engine.md`, `handlers.go:112`, `ledger.go:77`.

## Goals
- A given posting (one transaction, one kind) changes balances at most once, even under replay, redelivery, or two concurrent callers.
- A replayed step completes normally (it does not fail and does not double-post).

## Non-goals (separate slices)
Per-user idempotency keys (BE-H1), `compensation_failed` status and recovery of `compensating` (BE-H4), moving `step_executions` into the handler's transaction, the debit = credit constraint trigger, reversal uniqueness (BE-H3).

## Posting call sites (verified in code)
| Caller | Transaction id used | Kind |
|--------|--------------------|------|
| `workflow/handlers/handlers.go:112` (MOVE_FUNDS, forward) | the transaction | `forward` |
| same, compensating branch | the same transaction | `compensate` |
| `workflow/handlers/round_up.go:115` | the same, already posted, transaction | `round_up` |
| `wallet/callback.go:80`, `wallet/service.go:132`, `org/service.go:320` (top-ups) | the top-up transaction | `forward` |
| `reversal/reversal.go:164` | the new reversal transaction | `forward` |
Note: `round_up` posts a second set of legs on a transaction that was already posted, so uniqueness cannot be on the transaction id alone.

## Acceptance criteria
- **AC-1** Calling `Post` twice for the same (transaction, kind) writes entries and changes balances only the first time; the second call returns `ErrAlreadyPosted` and changes nothing.
- **AC-2** Re-running the `MOVE_FUNDS` handler after its first run committed (simulated crash before `recordAndEmit`) leaves balances unchanged and the step completes successfully.
- **AC-3** A compensating post is independent of the forward post of the same transaction (both can exist once), and a second compensating post changes nothing.
- **AC-4** `round_up` can still post once on a transaction that already has its `forward` posting.
- **AC-5** Two concurrent `Post` calls for the same (transaction, kind) result in exactly one effect and one `ErrAlreadyPosted`.
- **AC-6** The migration applies on a database with existing data (every already-posted transaction is back-filled) and its down migration reverses it; all existing tests still pass.
- **AC-7** The ledger stays append-only (the existing trigger still blocks UPDATE and DELETE).

## Invariants touched (constitution)
C-02 (append-only, at most one posting per transaction and kind), C-06 (consumers idempotent), C-14 (test first).

## Architecture impact: `amend`
- Pages: `architecture/crosscutting-concepts.md` (idempotency / replay safety), `architecture/runtime-views.md` (transfer sequence), `domain/ledger.md`, `domain/workflow-engine.md`, `reference/data-model.md`.
- ADR: `adr/0010-ledger-postings-guard.md` (proposed). It changes the signature of `ledger.Post` and adds a table owned by the ledger context.

## Constitution check
| ID | Touched? | Compliant? | How |
|----|----------|------------|-----|
| C-01 | no | n/a | amounts unchanged |
| C-02 | yes | yes, after change | unique posting guard; entries still append-only |
| C-03 | no | n/a | key scoping is a later slice |
| C-04 | yes | yes | lock order unchanged: the guard row is inserted after the wallet locks, in the same transaction |
| C-05 | no | n/a | |
| C-06 | yes | improves | replayed consumer step becomes a no-op |
| C-07, C-08, C-09, C-10, C-13 | no | n/a | |

## Open questions
1. After `ErrAlreadyPosted` the handler treats the step as successful. This assumes the first attempt's `CompleteTransaction` also committed or will on replay (`CompleteTransaction` only matches status `processing`). Verify with a replay test (TC-LED-004) before implementing.
2. Existing rows: back-fill kind `forward` for all posted transactions. Round-up entries made earlier on the same transactions are indistinguishable; this is harmless (a round-up never replays for an old transaction) but should be stated in the migration comment.