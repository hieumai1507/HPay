---
id: BUG-0003
title: Money-moving writes could run without an idempotency key (random server key) and money-request creation was not idempotent
status: fixed
severity: high
area: backend/transfer
tags: [idempotency, replay]
audit: [BE-H1]
found: 2026-11-21
fixed: 2026-11-21
commit: pending (fix is in the working tree, not committed)
regression_test: src/backend/test/integration/money/idempotency_test.go:TestIdempotency_MissingKeyRejectedInServices ; src/backend/test/integration/money/idempotency_test.go:TestIdempotency_MountedMoneyRoutesRequireKey ; src/backend/test/integration/money/idempotency_test.go:TestIdempotency_MoneyRequestCreate ; src/backend/test/integration/money/idempotency_test.go:TestIdempotency_MoneyRequestSplit
related: [BUG-0002]
---

# BUG-0003: money-moving writes without an idempotency key

## Symptom
- `ExecuteEngineRequest` called without a key generated `uuid.NewString()` itself, so a network retry of the same request created a second transaction (double payment).
- `POST /v1/money-requests` and `/split` had no `RequireIdempotencyKey` and no dedupe: a retry created duplicate payable requests (a split created a second group of N requests).

## Root cause
Idempotency was opt-in. The service layer treated an empty key as "no dedupe, invent one" (`transfer/service.go:344-347`), only 2 of the money routes mounted `RequireIdempotencyKey`, and the money-request service never stored a key at all. Wrong assumption: a missing key was a client convenience, not a contract violation (constitution C-03: keys are required on every money-moving write).

## Other places with the same flaw
`transfer.TransferToUser`, `wallet.Topup`, `moneyrequest.Pay` accepted an empty key (stored as NULL, never deduplicated); all now return `ErrIdempotencyKeyRequired`. Checked and intentionally unchanged: `confirm`/`verify` (state read only), decline/cancel/remind, vault create and round-up target (metadata only), bank callback (idempotent by `ref_code`). `org.Topup`/`CreateDisbursement` accept an empty key at the service (route enforces it); not changed (outside ownership).

## Fix
Router: `RequireIdempotencyKey()` added to `POST /v1/money-requests` and `/split`. Services: empty key rejected (`wallet.ErrIdempotencyKeyRequired`, mapped to 400 `ERR_IDEMPOTENCY_KEY_REQUIRED`); the random fallback removed. Money requests store `idempotency_key` and `request_hash` (migration 000015) and replay per requester; splits use `pg_advisory_xact_lock` to serialize concurrent retries. Signature change: `moneyrequest.Service.Create` and `CreateSplit` now take `idemKey`.

## Regression test
`TestIdempotency_MissingKeyRejectedInServices`, `TestIdempotency_MountedMoneyRoutesRequireKey` (real router, 4 routes with/without header, reads and decline unaffected), `TestIdempotency_MoneyRequestCreate`, `TestIdempotency_MoneyRequestSplit`.

## How to recognize this again
- Two transactions or two request groups with identical payload seconds apart from one user action.
- A service that falls back to `uuid.NewString()` for a missing key.
- A new money-writing POST in `router.go` without `RequireIdempotencyKey()`.

## Docs updated
Same set as BUG-0002, plus `domain/money-requests.md` (BR-MRQ-15) and `specs/w1-idempotency/spec.md` (route table).
