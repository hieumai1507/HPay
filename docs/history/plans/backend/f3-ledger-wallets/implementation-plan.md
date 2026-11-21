[← F2](../f2-auth/implementation-plan.md) · f3-ledger-wallets · [solution-architecture →](solution-architecture.md)

# F3 Ledger & Wallets — Implementation Plan

## 1. Scope & non-goals

| Ships | Not in F3 |
| --- | --- |
| Ledger service `Post()` — general double-entry (N legs, sum Dr = sum Cr), lock ordering by wallet_id, blocks negative balance on non-system wallets; seed system wallets + backfill wallets for existing users; create wallet on register/login (idempotent self-heal); sync **Topup** (bank mock); wallet API + history (keyset cursor) + transaction detail; idempotency by unique key (conflict → return the old transaction); per-txn limit by KYC tier; integration tests E01/E02/E03 | P2P transfer through the engine (F5 — engine F4 first); OTP for topup (mobile UX, enforced when notification is wired); aggregate daily limit (only per-txn in F3); reversal (F9); E15 bank callback (sync mock topup has no callback — E15 moves to F5 when an async flow exists) |

## 2. Task breakdown

| # | Task | Files | Size |
| --- | --- | --- | --- |
| T1 | Migration 000010: seed SYSTEM_CASH_IN + SYSTEM_FEE, backfill main wallet for existing users | db/migrations/000010_* | S |
| T2 | Rewrite wallets.sql with explicit columns (avoid scanning nullable-uuid owner_id) + ledger.sql + transactions.sql | internal/store/queries/* | M |
| T3 | `internal/ledger`: Post(ctx, tx, txnID, legs) — validate Dr/Cr balance, sorted locks, running balance_after | internal/ledger/*.go | L |
| T4 | `internal/wallet`: EnsureMainWallet, Topup (1 DB tx: txn record + ledger post + completed), History, Detail; per-txn limit by tier | internal/wallet/*.go | L |
| T5 | API: GET /v1/wallets, POST /v1/wallets/topup, GET /v1/transactions, GET /v1/transactions/:refCode + RequireIdempotencyKey middleware; call EnsureMainWallet on register/login | internal/api/* | M |
| T6 | Tests: unit (balanced legs, limit E13, self-transfer E14) + integration E01/E02/E03 (skipped if TEST_DATABASE_URL is missing); CI test job adds a postgres service + migrate | *_test.go, ci.yml | L |

## 3. Schema changes

Data migration only (000010): 2 system wallets + backfill. No structural change.

## 4. API changes

| Endpoint | Notes | Errors |
| --- | --- | --- |
| `GET /v1/wallets` | My wallets (main + vaults later) | — |
| `POST /v1/wallets/topup` | {amount} + mandatory Idempotency-Key header; bank mock succeeds immediately; duplicate key → 200 returning the old transaction (E01) | ERR_IDEMPOTENCY_KEY_REQUIRED, ERR_AMOUNT_INVALID, ERR_LIMIT_EXCEEDED |
| `GET /v1/transactions?cursor=&limit=` | Keyset by (created_at,id) desc — no offset (A02) | — |
| `GET /v1/transactions/:refCode` | Visible only to the transaction owner (sender/receiver wallet owner) | 404 when it is not the caller's |

## 5. Test plan

| ID | Test | Type |
| --- | --- | --- |
| E01 | `TestEdge_E01_IdempotentTopup`: 2 calls with the same key → 1 transaction, 1 pair of ledger entries, balance increased once | integration |
| E02 | `TestEdge_E02_ConcurrentOverdraw`: wallet 100, 2 goroutines withdraw 70 → exactly 1 succeeds, balance 30, never negative | integration |
| E03 | `TestEdge_E03_CrossTransferNoDeadlock`: A→B and B→A concurrently → both finish within the timeout, ledger balanced | integration |
| E13 | `TestEdge_E13_LimitByKycTier`: tier none exceeding 2M → ERR_LIMIT_EXCEEDED | unit |
| E14 | `TestEdge_E14_SelfTransferRejected`: legs in both directions on the same wallet are rejected by Post | unit |
| — | Post validate: legs with Dr/Cr mismatch, amount ≤ 0, < 2 legs → error | unit |

> **Deviation recorded:** the integration tests use `TEST_DATABASE_URL` (skipped when missing) instead of testcontainers as the gen-api-test skill describes — this reuses the existing docker-compose and adds no heavy dependency. It will be upgraded to testcontainers when the full testutil harness is built in F5. The gen-api-test skill will be updated if this approach proves good enough.

## 6. Rollout & risk

- Sync topup = money enters immediately without going through the engine — when F5 moves P2P to the engine, topup stays sync (decision in SA §5).

- Per-txn limits are hardcoded constants (none/pending: 2M, manual_review: 5M, verified: 100M) — moved into the fee/limit policy in the DB in F5 together with the fee engine.

- Running locally requires migration 000010: run `migrate ... up` before testing the API.

## 7. Execution checklist

- [x] gen-feature-plan · [ ] T1→T6 · [ ] fintech-review · [ ] commit `feat: F3 ledger and wallets - double-entry posting, topup, idempotency`
