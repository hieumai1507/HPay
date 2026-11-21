---
id: BUG-0011
title: Reversals can be duplicated, applied to a changed original, decided after expiry, and lose the approver's decision when the receiver has no funds
status: fixed
severity: high
area: backend/reversal
tags: [race, constraint, state-machine, money-math]
audit: [BE-H3, BE-H4]
found: 2026-11-21
fixed: 2026-11-21
commit: pending (fix is in the working tree, not committed)
regression_test: src/backend/test/integration/reversal_integrity_test.go (TestReversal_*, TestAdminDecide_ExpiredRequestRejected)
related: [BUG-0001]
---

# BUG-0011: Reversal integrity

(BUG-0002 to BUG-0009 are reserved for work running in parallel.)

## Symptom
Five defects in the reversal flow, each reproduced by a failing test before the fix:
1. A second reversal row for the same original was accepted by the database (the `one_reversal_per_txn` index that comments cited did not exist).
2. A reversal of a reversal transaction could be requested.
3. The executor posted a reversal for an original that was no longer `completed` unless a later guard happened to catch it.
4. `AdminDecide` accepted a decision after the request's deadline, unlike the org path.
5. When the receiver had spent the money, the executor's `ErrInsufficientBalance` rolled back the whole decision, including the approver's action; the request stayed `pending` for 24 hours with no signal.

## Root cause
- `reversal.go:106` and `queries/transactions.sql` described constraints that no migration created; protection rested on a non-atomic `ExistsPendingReversalForTxn` and on the globally unique key `rev-<id>`.
- `CreateRequest` checked only `status = completed`; reversals are born `completed`.
- The executor read the original without a lock and ignored the row count of `MarkTransactionReversed`.
- `approval/admin.go` had no expiry check; only `eligibility()` on the org path did.
- Executor errors were all-or-nothing: the approval engine had no way to say "this cannot be applied, park it".

## Other places with the same flaw
`approval_requests` allowed more than one pending request per subject for any subject type; fixed by the same partial unique index. The org path was checked: it already had the expiry check and an unrelated uniqueness (one request per org transaction), so it is unchanged.

## Fix
ADR-0013. Migration 000016 adds `needs_manual` to `approval_status` (own file). Migration 000017 adds the partial unique indexes `one_reversal_per_txn` and `one_pending_approval_per_subject`, guarded to fail with the offending ids if duplicates exist. `CreateRequest` rejects `type = reversal`. The executor locks the original (`GetTransactionForUpdate`), requires `completed`, and requires `MarkTransactionReversed` to affect one row. `AdminDecide` refuses expired requests and runs the executor in a savepoint: an `approval.NeedsManual` error rolls back the executor's work, keeps the approver's action, parks the request as `needs_manual` and writes the audit row `reversal.needs_manual`.

## Regression test
`TestReversal_ConcurrentRequestsOnlyOne` (8 goroutines), `TestReversal_SecondReversalRowRejectedByDB`, `TestReversal_ReversalOfReversalRejected`, `TestReversal_ExecutorRequiresCompletedOriginal`, `TestAdminDecide_ExpiredRequestRejected`, `TestReversal_ReceiverWithoutFundsGoesToNeedsManual` (also asserts balance equals ledger sum and that a second decision is refused). Mutation check: removing the reversal-of-reversal check, the insufficient-funds handling, the expiry check or the parked-status result each made a test fail. Removing only the explicit `completed` check does not fail a test because the row-count guard rolls back with the same result; the test pins the behavior, not the layer.
Full suite: 225 tests, 0 failures, `-race`.

## How to recognize this again
- Two `transactions` rows with the same `reversal_of`.
- An approval request that stays `pending` for 24 hours after an admin approved it, with an error in the API log (`insufficient balance`).
- A reversal posted for an original whose status is not `completed`.
- Admin decisions timestamped after `expires_at`.

## Docs updated
`domain/reversal.md` (BR-REV-03, 04, 12 to 15; known gaps), `domain/approval-maker-checker.md` (BR-APR-18, 19), `reference/data-model.md`, `constitution.md` (C-05, C-13), `adr/0013-*`, audit report.

## Not done
Resolution of a `needs_manual` request (retry, write-off, debt record) and admin UI for the new status; the same all-or-nothing behavior for workflow compensation (BE-H4, workflow part).