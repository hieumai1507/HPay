---
title: Wave 2 slice - reversal integrity (double reversal, expiry, receiver without funds)
type: spec
status: proposed
last_verified: 2026-11-21
audit: [BE-H3, BE-H4]
related:
  - ../../domain/reversal.md
  - ../../domain/approval-maker-checker.md
  - ../../adr/0009-reversal-failure-needs-manual-and-fail-closed-fraud.md
  - ../../adr/0013-reversal-uniqueness-and-needs-manual.md
---

# Wave 2 slice: reversal integrity (BE-H3, BE-H4)

## Problem (verified in `docs/domain/reversal.md` and the code)
1. **Double reversal is not enforced by the schema.** The comments in `reversal.go:106` and `queries/transactions.sql` cite `UNIQUE(subject_type, subject_id)` and `one_reversal_per_txn`, but no migration creates them. The only protection is a non-atomic `ExistsPendingReversalForTxn` check plus the globally unique idempotency key `rev-<id>`, so a race ends in a raw 23505 inside the finalize transaction.
2. **The executor does not re-check the original.** It does not require `status = completed` under a lock, and ignores the row count of `MarkTransactionReversed`.
3. **A reversal of a reversal is allowed** (`CreateRequest` only checks `completed`; reversal transactions are born `completed`).
4. **`AdminDecide` ignores expiry**, unlike the org `eligibility()` path.
5. **Receiver without funds (BE-H4):** the executor's debit of the receiver returns `ErrInsufficientBalance`, which rolls back the whole `AdminDecide`; the request stays `pending` until it expires and nobody is told why.

## Product decision already taken (ADR-0009, user, 2026-11-21)
A reversal that cannot complete goes to a `needs_manual` status with an audit record. It never produces a negative balance and never loses the approver's decision.

## Acceptance criteria
- **AC-1** Two concurrent reversal requests for the same transaction: exactly one is created; the other returns `ErrReversalPending`.
- **AC-2** Two reversal transactions can never exist for one original (database constraint), even if the executor runs twice.
- **AC-3** A reversal request for a transaction of type `reversal` is rejected.
- **AC-4** The executor locks the original and requires `completed`; an original that changed state fails the finalize with a clear error and changes nothing.
- **AC-5** `AdminDecide` on an expired request returns the same expired error as the org path and does not execute.
- **AC-6** If the receiver cannot cover the reversal, the approval ends as `needs_manual`: the approver's signature and an audit row are persisted, no ledger entry and no reversal transaction are created, the original stays `completed`, and no balance changes.
- **AC-7** A `needs_manual` request cannot be decided again; resolving it is a manual operation outside this slice (documented in the runbook).
- **AC-8** Existing reversal tests and the full suite still pass; migration up/down/up works on data.

## Invariants touched
C-02, C-05 (state only through the owning function), C-13 (fail closed). Architecture impact: `amend` (approval state machine gains `needs_manual`; `transactions` and `approval_requests` get partial unique indexes). ADR-0013 (proposed).

## Constitution check
| ID | Touched | Compliant | How |
|----|---------|-----------|-----|
| C-02 | yes | yes | reversal posts once via the ledger guard (ADR-0010) and once per original via a unique index |
| C-05 | yes | yes | new status only through the approval service transition |
| C-13 | yes | yes | failure holds the request for manual handling, no negative balance |
| C-14 | yes | yes | tests first |

## Open questions
1. Who resolves a `needs_manual` request and how (retry after the receiver is funded, write-off, debt account)? This slice only parks it safely and records it; a resolution flow is a separate feature (ADR-0009 follow-up).
2. Migration numbering: the idempotency slice owns `000015`; this slice uses the next two numbers (an enum value must be added in its own migration, as 000011 did, then a second file for the indexes).