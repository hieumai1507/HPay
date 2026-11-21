---
title: Wave 1 slice 2 - idempotency per user, bound to the request
type: spec
status: done
last_verified: 2026-11-21
audit: [BE-H1]
related:
  - ../../domain/transfer-and-fees.md
  - ../../domain/money-requests.md
  - ../../domain/wallet.md
  - ../../adr/0011-idempotency-per-user-bound-to-request.md
---

# Wave 1 slice 2: idempotency per user, bound to the request (BE-H1)

## Problem
`transactions.idempotency_key` is globally `UNIQUE` (migration 000002:57). When a key repeats, transfer, wallet and money-request code returns the earlier transaction regardless of who sent it or what it contained: user B reusing user A's key receives A's transaction, and the same user changing the amount still gets the old result with no error. `ExecuteEngineRequest` invents a random key when the client sends none, so a retry creates a second transaction. `RequireIdempotencyKey` is mounted on two routes only; `POST /v1/money-requests` and `/split` had no idempotency at all.

## Goals
- A key belongs to the user who sent it: `UNIQUE (initiated_by, idempotency_key)`.
- A key is bound to the request: a stored `request_hash` (SHA-256 of a canonical encoding). Same key and same hash returns the original result; same key and a different hash returns 409 `ERR_IDEMPOTENCY_KEY_REUSED`.
- A money-moving write without a key is rejected (HTTP 400 `ERR_IDEMPOTENCY_KEY_REQUIRED` at the route, `ErrIdempotencyKeyRequired` at the service). No server-generated fallback.
- The migration applies on a populated database and has a working down.

## Non-goals
Org package (`org.Topup`, `org.CreateDisbursement`): outside this slice's file ownership, see "Follow-ups". Key expiry/retention, a response cache (the original result is rebuilt from the stored row), hash binding for the legacy `NULL`-hash rows.

## Mounted routes: which get `RequireIdempotencyKey`
| Route | Moves money or creates a money-bearing object | Key required |
|-------|-----------------------------------------------|--------------|
| `POST /v1/transactions/request` (engine: P2P, top-up, vault deposit/withdraw, money-request pay) | yes | already, kept |
| `POST /v1/org/:id/transactions` | yes | already, kept |
| `POST /v1/money-requests` | creates a payable request | **added** |
| `POST /v1/money-requests/split` | creates N payable requests | **added** |
| `POST /v1/transactions/confirm`, `/verify` | no (report state only; they never touch the ledger) | no |
| `POST /v1/money-requests/:id/decline`, `/cancel`, `/remind` | state changes, no money | no |
| `POST /v1/vaults` (create), `PATCH /v1/vaults/:id/round-up-target` | metadata only; vault money moves through the engine (VAULT_SAVINGS) | no |
| `POST /v1/kyc/submit`, `/v1/auth/*`, `/v1/qr/resolve`, `/v1/admin/*`, `/v1/org` (create), members, policies, approvals | no money | no |
| `/internal/mock-bank/callback` | idempotent by `ref_code` (E15) | no |
Not mounted (so nothing to guard, but the services enforce the key anyway): `wallet.Service.Topup`, `transfer.Service.Transfer*`, `moneyrequest.Service.Pay`.

## Acceptance criteria
- **AC-1** Same user, same key, same payload: the original result is returned (`duplicate`), exactly one transaction exists and balances move once (transfer, engine, top-up, money-request pay).
- **AC-2** Same user, same key, different payload (amount, receiver, note, service, extra params; or another money request on pay): HTTP 409 `ERR_IDEMPOTENCY_KEY_REUSED`, nothing is created.
- **AC-3** Two users with the same key are independent: neither receives or collides with the other's transaction; each can still replay their own.
- **AC-4** A money-moving write without a key is rejected: 400 `ERR_IDEMPOTENCY_KEY_REQUIRED` on every mounted money route; the services return `ErrIdempotencyKeyRequired`; the random fallback is gone.
- **AC-5** Concurrent requests with the same key create exactly one transaction and all callers get the same result (single and split).
- **AC-6** Money requests (create, split) are idempotent per requester with the same replay/409 rules; a split replays all its rows in the original order under the original `split_group_id`.
- **AC-7** Migration 000015 applies on a database that already has transactions, leaves old rows with `request_hash = NULL` (treated as "unknown, accept replay"), allows another user to reuse an old key, still forbids the same user, and its down migration works (and fails loudly if two users share a key by then).
- **AC-8** Read endpoints, auth routes and non-money writes do not require the header.

## Invariants touched
C-03 (this slice implements it), C-02/C-06 unaffected (posting guard untouched), C-14 (tests first for the router, money-request and error-map parts).

## Schema / API delta
- `transactions`: drop `transactions_idempotency_key_key`, add `request_hash VARCHAR(64) NULL`, add `uq_txn_user_idempotency UNIQUE (initiated_by, idempotency_key)` (`initiated_by` is `NOT NULL` since 000002, NULL keys stay unconstrained).
- `money_requests`: add `idempotency_key VARCHAR(80)`, `request_hash VARCHAR(64)`, partial unique index `uq_money_req_user_idempotency (requester_id, idempotency_key) WHERE idempotency_key IS NOT NULL AND split_group_id IS NULL`. Splits (many rows per key) are serialized by `pg_advisory_xact_lock`.
- New error code `ERR_IDEMPOTENCY_KEY_REUSED` (409); `ERR_IDEMPOTENCY_KEY_REQUIRED` is now also produced by the error map. OpenAPI: header added to money-request create and split, 409 documented, `IdempotencyKey` parameter described.
- Queries: `InsertTransaction` takes `request_hash`; new `GetUserTransactionByIdempotencyKey`; `GetTransactionByIdempotencyKey` kept as LEGACY for `internal/org` only; `InsertMoneyRequest` takes key and hash; new `ListMoneyRequestsByIdempotencyKey`.

## Hash definition
SHA-256 over length-prefixed fields (`len:value;`), operation tag first.
| Operation | Fields |
|-----------|--------|
| p2p transfer | receiver id, amount, note |
| engine request | service code (upper-cased, default P2P_TRANSFER), receiver id, receiver phone, amount, note, extra_params JSON (keys sorted by `encoding/json`) |
| top-up | amount |
| money request create | payer id, amount, note |
| money request split | total, note, then (payer id, share) per payer in request order |
| money request pay | request id |

## Edge-case IDs
E01 (idempotent top-up) extended to per-user scope. Test cases: `test-cases.md`.

## Architecture impact: `amend`
Pages that change: `architecture/crosscutting-concepts.md` (section 2 Idempotency), `domain/transfer-and-fees.md`, `domain/money-requests.md`, `domain/wallet.md`, `reference/data-model.md`, `reference/api.md`, `reference/error-codes.md`. ADR: `adr/0011-idempotency-per-user-bound-to-request.md` (proposed). Pages read to conclude this: `architecture/crosscutting-concepts.md`, `domain/*` above, `constitution.md`.

## Constitution check
| ID | Touched? | Compliant? | How |
|----|----------|------------|-----|
| C-01 | no | n/a | amounts unchanged; hash encodes them as decimal strings |
| C-02 | no | n/a | |
| C-03 | yes | yes, after change | per-user, hash-bound, mandatory key |
| C-04 | no | n/a | the pay path takes the existing request row lock first; the replay lookup runs under it |
| C-05 | yes | yes | no new data crosses a trust boundary; the other user's row is never selected |
| C-06 | no | n/a | |
| C-07 | yes | yes | error codes registered in errmap and openapi, sync check run |
| C-08 | yes | yes | migration has a tested down, applies on populated data |
| C-13 | yes | yes | replay is rebuilt from the stored row; no extra PII |

## Follow-ups (outside this slice's ownership)
1. `internal/org` (`Topup`, `CreateDisbursement`) still does a key-only, un-hashed replay lookup through the LEGACY query `GetTransactionByIdempotencyKey` (oldest row with that key, any user). Same-user payload mismatch is not detected there, and a residual leak remains: if user A and user B both used key K, B repeating K hits the per-user 23505 and the legacy lookup can return A's oldest row. The org service must adopt `wallet.FindReplay` and `wallet.RequestHash`, then the LEGACY query can be dropped. BE-H1 is therefore fixed for transfer, engine, wallet and money requests, and open for the org package (tracked in the audit report).
2. `cmd/seed` inserts transactions with literal keys and no `request_hash`; fine (NULL = unknown).
3. Mobile and admin clients must send a stable `Idempotency-Key` per user action on money-request create and split (the header is now mandatory there).
