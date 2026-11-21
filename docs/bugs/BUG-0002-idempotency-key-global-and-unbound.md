---
id: BUG-0002
title: Idempotency-Key is global and not bound to the request, so another user's or a changed request returns a stale transaction
status: fixed
severity: high
area: backend/transfer
tags: [idempotency, replay, authz, constraint, migration]
audit: [BE-H1]
found: 2026-11-21
fixed: 2026-11-21
commit: pending (fix is in the working tree, not committed)
regression_test: src/backend/test/integration/money/idempotency_test.go:TestIdempotency_TwoUsersSameKeyAreIndependent ; src/backend/test/integration/money/idempotency_test.go:TestIdempotency_Transfer_SameKeyDifferentPayloadRejected ; src/backend/test/integration/money/idempotency_migration_test.go:TestMigration000015_UpDownBackfill
related: [BUG-0003]
---

# BUG-0002: Idempotency-Key is global and not bound to the request

## Symptom
- User B sending `Idempotency-Key: K` after user A used `K` received A's transaction (ref_code, amount, fee, status) in the response, and B's own request was dropped. A data leak and a lost payment.
- The same user repeating `K` with a different amount or receiver got the old result with HTTP 200 and no error, so the client believed the new request succeeded.

## Root cause
`transactions.idempotency_key` was `UNIQUE` globally (migration 000002:57). Every service treated "23505 on a constraint named like idempotency" as "return the transaction with this key" and looked it up by key alone (`GetTransactionByIdempotencyKey WHERE idempotency_key = $1`), with no owner and no comparison of what the request contained. Wrong assumption: the key was treated as a globally unique request id instead of a client-chosen value scoped to its owner.

## Other places with the same flaw
- `transfer.TransferToUser`, `transfer.ExecuteEngineRequest`, `wallet.Topup`, `moneyrequest.Pay`: fixed (same helper).
- `org.Topup`, `org.CreateDisbursement`: same pattern; fixed afterwards in the same working tree (2026-11-21) with the same `FindReplay` contract, tests in `test/integration/org/org_idempotency_test.go`.
- `reversal` uses a server-built key (`rev-<txn id>`) and `cmd/seed` uses literal keys: not client-controlled, left as is.

## Fix
Migration 000015: drop the global unique constraint, add `request_hash` and `UNIQUE (initiated_by, idempotency_key)`. New helpers in `wallet/idempotency.go` (`RequestHash`, `FindReplay`, `IsIdempotencyConflict`); the transfer, engine, top-up and pay paths look up `(user, key)`, compare the hash, answer 409 `ERR_IDEMPOTENCY_KEY_REUSED` on mismatch and resolve concurrent races through the unique index. Old rows keep a NULL hash and accept replay. Decision record: ADR-0011.

## Regression test
- `TestIdempotency_TwoUsersSameKeyAreIndependent`: B with A's key gets its own transaction and balance.
- `TestIdempotency_Transfer_SameKeyDifferentPayloadRejected`, `TestIdempotency_Topup_DifferentAmountRejected`, `TestIdempotency_EngineHTTP_ReplayConflictAndIsolation`: 409 / `ErrIdempotencyKeyReused` for changed amount, receiver or note.
- `TestIdempotency_ConcurrentSameKeyOneTransaction`: 8 concurrent calls, one transaction, one debit.
- `TestMigration000015_UpDownBackfill`: applies on populated data, old rows NULL hash, other user may reuse an old key, down works.

## How to recognize this again
- A response whose `ref_code`/amount does not match what the caller just sent.
- 23505 errors mentioning an idempotency constraint in logs for different users.
- A new idempotent endpoint that does `WHERE idempotency_key = $1` without `initiated_by`.

## Docs updated
`domain/transfer-and-fees.md` (BR-TRF-02, -03, -03a, -03b), `domain/wallet.md` (BR-WAL-25), `domain/money-requests.md` (BR-MRQ-10), `reference/data-model.md`, `reference/api.md`, `reference/error-codes.md`, `architecture/crosscutting-concepts.md`, `constitution.md` (C-03), `audit/AUDIT-REPORT.md`, ADR-0011.
