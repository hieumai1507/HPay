---
title: Test cases - idempotency per user, bound to the request
type: spec
status: done
spec: spec.md
---

# Test cases

All automated cases are in `src/backend/test/integration/idempotency_test.go` and `idempotency_migration_test.go`. Run: `.\scripts\test-backend.ps1 -GoArgs "./test/integration -run (TestIdempotency|TestMigration000015) -race -count=1 -v"`.

| Test ID | AC | Type | Priority | Executor | Steps | Expected | Automated as |
|---------|----|------|----------|----------|-------|----------|--------------|
| TC-IDM-001 | AC-1 | Happy / replay | P0 | AUTO-API | User A transfers 100,000 with key K, then repeats the identical call | Second call returns the same `ref_code` with `duplicate`; one transaction row; sender debited once | `TestIdempotency_Transfer_ReplaySameUserSamePayload` |
| TC-IDM-002 | AC-2 | Negative / security | P0 | AUTO-API | A transfers with K, then reuses K changing amount, receiver, or note | Each returns `ErrIdempotencyKeyReused`; still one transaction | `TestIdempotency_Transfer_SameKeyDifferentPayloadRejected` |
| TC-IDM-003 | AC-3 | Security (isolation) | P0 | AUTO-API | A tops up 300,000 with K; B tops up 700,000 with the same K; A repeats | B gets its own new transaction and balance 700,000 (no A data); A's replay returns A's transaction | `TestIdempotency_TwoUsersSameKeyAreIndependent` |
| TC-IDM-004 | AC-4 | Negative | P0 | AUTO-API | Call engine request, transfer, top-up, money-request create, split and pay with an empty key | `ErrIdempotencyKeyRequired` from each; nothing created | `TestIdempotency_MissingKeyRejectedInServices` |
| TC-IDM-005 | AC-5 | Concurrency | P0 | AUTO-API | 8 goroutines transfer with the same K and payload | All return the same `ref_code`; one transaction; sender debited once | `TestIdempotency_ConcurrentSameKeyOneTransaction` |
| TC-IDM-006 | AC-1, AC-2, AC-3 | Happy / negative / security | P0 | AUTO-API | HTTP `POST /v1/transactions/request`: A with K; A with K and different amount; B with K | 200; 409 `ERR_IDEMPOTENCY_KEY_REUSED`; 200 with B's own transaction | `TestIdempotency_EngineHTTP_ReplayConflictAndIsolation` |
| TC-IDM-007 | AC-6, AC-3 | Happy / negative | P1 | AUTO-API | Create a money request with K twice; with a different amount; as another requester with K | Same request id, one row; `ErrIdempotencyKeyReused`; independent request for the other user | `TestIdempotency_MoneyRequestCreate` |
| TC-IDM-008 | AC-6, AC-5 | Happy / negative | P1 | AUTO-API | Create a 2-payer split with K twice; then with a different total; then with different payers | Same `split_group_id` and request ids in the original order, 2 rows; both variants `ErrIdempotencyKeyReused` | `TestIdempotency_MoneyRequestSplit` |
| TC-IDM-009 | AC-1, AC-2 | Happy / negative | P0 | AUTO-API | Payer pays request R1 with K twice; then pays R2 with K | Replay returns the original with `duplicate`; R2 gets `ErrIdempotencyKeyReused`; one transaction | `TestIdempotency_MoneyRequestPay` |
| TC-IDM-010 | AC-2 | Negative | P1 | AUTO-API | Wallet top-up with K and 100,000, then K and 200,000 | `ErrIdempotencyKeyReused`; balance stays 100,000 | `TestIdempotency_Topup_DifferentAmountRejected` |
| TC-IDM-011 | AC-4, AC-8 | Negative / boundary | P0 | AUTO-API | Real router: POST the 4 mounted money routes without the header; with the header; GET `/v1/wallets`; POST `/money-requests/:id/decline` without it | 400 `ERR_IDEMPOTENCY_KEY_REQUIRED` without; passes the middleware with; reads and decline never ask for it | `TestIdempotency_MountedMoneyRoutesRequireKey` |
| TC-IDM-012 | AC-7 | Migration | P0 | AUTO-API | Scratch DB at 000014 with legacy rows (keyed and NULL-keyed), apply 000015 | Success; old `request_hash` NULL; second user can reuse an old key; same user cannot; NULL keys unconstrained; `money_requests` columns exist | `TestMigration000015_UpDownBackfill` |
| TC-IDM-013 | AC-7 | Migration (down) | P0 | AUTO-API | Down while two users share a key; remove the clash; down again; up again | First down fails loudly; second succeeds and drops `request_hash`; up re-applies | `TestMigration000015_UpDownBackfill` |
| TC-IDM-014 | AC-1 | Regression | P0 | AUTO-API | Existing E01 test (same user, same key, same payload top-up) | Still one transaction, credited once | `TestEdge_E01_IdempotentTopup` |
| TC-IDM-015 | AC-4 | Regression | P1 | AUTO-API | Existing HTTP tests: engine route without a header | 400 | `TestTransactionRequestHTTP_MissingIdempotencyKeyRejected` |

Edge IDs: E01 (TC-IDM-014, -003). Not covered by an automated case: retention of keys, org package replay (follow-up 1 in `spec.md`).
