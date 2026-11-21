---
id: ADR-0010
title: Guard ledger postings with a unique (transaction, kind) record
status: proposed
date: 2026-11-21
supersedes: []
superseded_by: []
spec: specs/w1-ledger-and-idempotency/spec.md
affects:
  - architecture/crosscutting-concepts.md
  - architecture/runtime-views.md
  - domain/ledger.md
  - domain/workflow-engine.md
  - reference/data-model.md
constitution: [C-02, C-06]
---

# ADR-0010: Guard ledger postings with a unique (transaction, kind) record

Status: **proposed**. Needs the user's approval before any code is written.

## Context
`ledger.Post` can be applied twice to the same transaction, because the worker records a step's completion in a different database transaction than the handler's money movement, and event delivery is at-least-once (audit BE-C1, `domain/ledger.md`, `domain/workflow-engine.md`). `ledger_entries` cannot carry a simple unique key on `transaction_id`, because compensation and round-up legitimately post again on a transaction that was already posted.

## Options considered
| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| **A. New table `ledger_postings (transaction_id, kind)` with a primary key; `Post(ctx, tx, txnID, kind, legs)` inserts the guard row first and returns `ErrAlreadyPosted` on conflict** | Does not touch the append-only `ledger_entries`; explicit; one row per posting; easy to backfill; the guard rolls back together with the posting | Signature change touches 8 call sites; a new table to document | Low to medium |
| B. Add column `posting_kind` to `ledger_entries` and a unique index on `(transaction_id, posting_kind, wallet_id, direction)` | No new table | Entry-level uniqueness breaks if a posting ever has two legs on the same wallet and direction; alters the hottest table; backfill is heavier | Medium |
| C. Move step recording into the handler's transaction (claim row) only | Fixes the crash window at its root for workflow steps | Does not protect other callers (top-ups, reversals) or concurrent callers; larger change in the engine | Higher; worth doing later as a second layer |
| D. Do nothing in the database; rely on handler-level checks | No migration | Check-then-act race; the original defect remains | Not acceptable |

## Decision
Adopt **A**. Kinds are `forward`, `compensate` and `round_up`. `Post` inserts the guard row inside the caller's transaction after the wallets are locked and before the entries. On a duplicate it returns `ErrAlreadyPosted`; workflow handlers treat that as "this posting already happened" and complete the step successfully. Back-fill `forward` for every transaction that already has ledger entries. Option C remains a follow-up layer, tracked separately.

## Constitution check
- C-02: strengthens it (at most one posting per transaction and kind); the append-only trigger is untouched.
- C-04: lock order is unchanged; the guard insert happens after the ordered wallet locks.
- C-06: a replayed consumer step becomes a no-op.
No deviation.

## Consequences
- Easier: replay and redelivery of money steps are safe; the invariant is checkable in SQL.
- Harder: every new posting site must pass a kind; tests that call `Post` directly need updating; one more table to back up and document.
- Risk: treating `ErrAlreadyPosted` as success can hide a bug where a different payload is posted for the same (transaction, kind). Mitigation: store the total amount posted in the guard row and compare on conflict (proposed: add a `total_amount` column; to be confirmed in review).
- Follow-ups: step recording in the handler transaction (option C), debit = credit constraint trigger, reversal uniqueness.

## Documentation to update
`domain/ledger.md`, `domain/workflow-engine.md`, `reference/data-model.md`, `architecture/crosscutting-concepts.md`, `architecture/runtime-views.md`, `adr/README.md`.