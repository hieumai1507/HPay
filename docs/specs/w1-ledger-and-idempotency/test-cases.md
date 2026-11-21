---
title: Test cases - ledger posting at most once
type: spec
status: proposed
spec: spec.md
---

# Test cases (design, before any code)

Not yet added to `docs/qa/test-cases/*.xlsx`: they are added through skill `hpay-qa-testcases` (catalog plus `merge_results.py --allow-new-ids`) once this list is approved.

| Test ID | AC | Type | Priority | Executor | Steps | Expected | Automated as |
|---------|----|------|----------|----------|-------|----------|--------------|
| TC-LED-001 | AC-1 | Happy / Edge | P0 | AUTO-API | Create a funded wallet pair and a transaction. Post legs twice with the same (txn, kind). | First call writes N entries and moves balances; second returns `ErrAlreadyPosted`, entry count and balances unchanged | `internal/ledger` `TestPost_SecondCallSameKindIsRejected` |
| TC-LED-002 | AC-5 | Edge (concurrency) | P0 | AUTO-API | Two goroutines `Post` the same (txn, kind) at once, run with `-race` | Exactly one succeeds, one gets `ErrAlreadyPosted`; balances moved once | `TestPost_ConcurrentSameKindOnce` |
| TC-LED-003 | AC-3 | Edge | P0 | AUTO-API | Post `forward`, then `compensate`, then `compensate` again for one txn | forward once, compensate once, second compensate rejected; net effect is zero | `TestPost_CompensateIndependentAndOnce` |
| TC-LED-004 | AC-2 | Edge (replay) | P0 | AUTO-API | Run MOVE_FUNDS for a transaction through the engine, then call the handler again with the same input (as a redelivery would) | Balances unchanged after the second run; step returns success | `test/integration` `TestEdge_E06_DuplicateStepNoDoublePost` |
| TC-LED-005 | AC-4 | Edge | P1 | AUTO-API | Complete a transfer, then run the round-up step for the same transaction | Round-up legs post once on top of the forward posting; a second round-up run does nothing | `TestRoundUp_AfterForwardPosting` |
| TC-LED-006 | AC-6 | Boundary | P0 | AUTO-API | Apply migration to a database that already contains posted transactions; run up, down, up | Every posted transaction has a posting row; down reverses cleanly; no data loss | CI `migrate` job plus `TestMigration_BackfillsExistingPostings` |
| TC-LED-007 | AC-7 | Security / Integrity | P1 | AUTO-API | Attempt UPDATE and DELETE on `ledger_entries` | Both rejected by the trigger | existing trigger test (confirm it exists; add if not) |
| TC-LED-008 | AC-1, AC-6 | Negative | P1 | AUTO-API | Post with a failing leg (insufficient balance) | No posting row is left behind (the guard row rolls back with the transaction); a retry can post | `TestPost_FailedPostingLeavesNoGuard` |
| TC-LED-009 | AC-2 | Edge (crash window) | P1 | HUMAN or AUTO | Kill the worker between handler commit and `recordAndEmit` in a local run, restart | After restart the transaction completes once; wallet balances equal the single transfer | manual script in `docs/guides/debug-a-stuck-transaction.md`; automate if feasible |

Coverage: every AC-1 to AC-7 has at least one case.