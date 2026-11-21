---
title: Test cases - reversal integrity
type: spec
status: proposed
spec: spec.md
---

# Test cases (designed before code)

| Test ID | AC | Type | Priority | Executor | Steps | Expected | Automated as |
|---------|----|------|----------|----------|-------|----------|--------------|
| TC-REV-001 | AC-1 | Edge (concurrency) | P0 | AUTO-API | 8 goroutines call `CreateRequest` for one completed transaction | exactly 1 request, 7 get `ErrReversalPending` | `test/integration` `TestReversal_ConcurrentRequestsOnlyOne` |
| TC-REV-002 | AC-2 | Integrity | P0 | AUTO-API | insert a second `transactions` row with the same `reversal_of` | rejected by the unique index | `TestReversal_SecondReversalRowRejectedByDB` |
| TC-REV-003 | AC-3 | Negative | P1 | AUTO-API | complete a reversal, then request a reversal of the reversal transaction | `ErrNotReversible` | `TestReversal_ReversalOfReversalRejected` |
| TC-REV-004 | AC-4 | Edge | P0 | AUTO-API | request a reversal, change the original away from `completed`, approve | finalize fails with a clear error; no ledger entries, no reversal row | `TestReversal_ExecutorRequiresCompletedOriginal` |
| TC-REV-005 | AC-5 | Negative | P0 | AUTO-API | create a request, set `expires_at` in the past, `AdminDecide(approve)` | expired error; nothing executed | `internal/approval` `TestAdminDecide_ExpiredRequestRejected` |
| TC-REV-006 | AC-6 | Edge | P0 | AUTO-API | transfer to B, B spends everything, request and approve a reversal | request becomes `needs_manual`; approver action and audit row saved; no new ledger entries; no reversal transaction; balances unchanged | `TestReversal_ReceiverWithoutFundsGoesToNeedsManual` |
| TC-REV-007 | AC-7 | Negative | P1 | AUTO-API | decide a `needs_manual` request again | `ErrAlreadyFinalized` | `TestReversal_NeedsManualCannotBeDecidedAgain` |
| TC-REV-008 | AC-6 | Integrity | P0 | AUTO-API | after TC-REV-006: wallet balance equals ledger sum for both wallets and system fee wallet | equal | assertion inside TC-REV-006 |
| TC-REV-009 | AC-8 | Boundary | P0 | AUTO-API | migration up, down, up on a database with a completed reversal and a pending request | succeeds; existing rows unaffected | CI `migrate` job plus `TestMigration_ReversalIndexesBackfillSafe` |
| TC-REV-010 | AC-1..AC-6 | Regression | P0 | AUTO-API | existing `TestReversal*`, `TestEdge_E10*`, approval tests | unchanged | existing tests |