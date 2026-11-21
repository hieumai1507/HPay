---
title: Transfers, fees and offer limits
type: domain
status: verified-against-code
last_verified: 2026-11-21
code:
  - src/backend/internal/transfer/service.go
  - src/backend/internal/fee/fee.go
  - src/backend/internal/offerguard/offerguard.go
  - src/backend/internal/product/service.go
  - src/backend/internal/api/transaction_engine_handler.go
  - src/backend/internal/api/transfer_handler.go
  - src/backend/internal/workflow/handlers/handlers.go
  - src/backend/internal/store/queries/fee.sql
  - src/backend/internal/store/queries/offer_guard.sql
  - src/backend/db/migrations/000004_offer_factory_policies.up.sql
  - src/backend/db/migrations/000006_service_dispatch_user_offers.up.sql
related:
  - domain/ledger.md
  - domain/wallet.md
audit:
  - BE-C1
  - BE-C2
  - BE-H1
  - BE-H2
  - BE-H4
  - BE-M5
---

# Transfers, fees and offer limits

## 1. Purpose
Turns a user request (P2P transfer, money-request payment, bank top-up, vault deposit/withdraw) into a `transactions` row plus a started workflow instance, after checking service availability, offer limits and balance, and after freezing the fee on the row. The workflow steps (VALIDATE, FRAUD_CHECK, MOVE_FUNDS, CONFIRM, ROUND_UP) then do the money movement through `ledger.Post`. This doc does NOT cover the workflow engine internals, fraud rules, reversal, org disbursement or approvals. `internal/product` is only an admin CRUD facade (organizations, services, offers, audit logs, workflow instances, fraud flags listing, `product/service.go`) and holds no transfer logic.

## 2. Key concepts
- **Service**: a row in `services` with `code`, `status`, and dispatch columns `workflow_name`, `ledger_txn_type`, `wallet_strategy` (000006:15-25). Seeded: `P2P_TRANSFER`, `MONEY_REQUEST` (both p2p/user_to_user), `TOPUP_BANK` (topup/syscash_to_user), `VAULT_SAVINGS` (vault_move/user_to_vault), `ORG_DISBURSEMENT` (org_disbursement/org_to_user).
- **Engine request**: `POST /v1/transactions/request`, the single mounted entry point for user money movement.
- **Offer / offer_service**: package assigned to a user (`user_offers`) with per-service limits `min_amount`, `max_amount_per_txn`, `daily_limit`, `monthly_limit`, `interval_seconds` (0 means disabled). Fallback package when the user has none: `OFFER_CUST_L2` (override env `DEFAULT_OFFER_CODE`) (`offerguard.go:44,55-58`).
- **Auth matrix** (`offer_auth_matrix`): amount ranges mapped to `none|pin|otp|2fa`.
- **Fee policy** (`fee_policies`): per `txn_type` and optional `kyc_status`, with `fixed_fee`, `percent_bp`, `min_fee`, `max_fee`.
- **Ref code**: `MP` + 12 upper-hex chars (`wallet.NewRefCode`, see wallet.md BR-WAL-09).

## 3. Business rules

| ID | Rule | Enforced by (code or DB) | Test |
|----|------|--------------------------|------|
| BR-TRF-01 | Amount must be > 0 (service and HTTP binding `gt=0`). | `transfer/service.go:ExecuteEngineRequest` (278-280); `api/transaction_engine_handler.go:24` | `TestQA_DisbursementZeroOrNegativeAmountRejected` (org path only) |
| BR-TRF-02 | `POST /v1/transactions/request` requires header `Idempotency-Key` (1..64 chars). | `api/middleware.go:RequireIdempotencyKey` (83-93), `router.go:118` | `TestTransactionRequestHTTP_MissingIdempotencyKeyRejected` |
| BR-TRF-03 | A repeated idempotency key returns the stored transaction (status, amount, fee) instead of creating a new one. Detection is a unique violation whose constraint name contains `idempotency`. | `service.go:368-386`; `transactions.idempotency_key UNIQUE` (000002:57) | `TestTransactionRequestHTTP_IdempotencyKeyHeaderDedupes` |
| BR-TRF-04 | Service code is upper-cased/trimmed; empty means `P2P_TRANSFER`; an unknown code silently falls back to `P2P_TRANSFER`. | `service.go:283-294` | none |
| BR-TRF-05 | A service whose status is not `active` is refused. | `service.go:296-298` | none |
| BR-TRF-06 | With `ENGINE_DYNAMIC_DISPATCH != "false"` (default on) txn type, workflow definition and wallet strategy come from the service row; otherwise legacy P2P (`user_to_user`, `s.defName`). | `NewService` (49); `service.go:302-309` | none |
| BR-TRF-07 | OfferGuard runs first (unless env `OFFERGUARD_ENABLED=="false"`, default on). When off the auth method defaults to `pin` and no limits apply. | `service.go:311-320`; `NewService` (51) | none |
| BR-TRF-08 | A self-transfer (receiver == sender) is rejected with `ErrSelfTransfer`. | `service.go:planWallets` (488-490) | none (ledger BR-LED-04 has a unit test) |
| BR-TRF-09 | Receiver is resolved by `receiver_id` or `receiver_phone`; neither gives an error; unknown phone gives `ErrReceiverNotFound`. | `service.go:planWallets` (472-487) | none |
| BR-TRF-10 | Wallet strategies wired: `user_to_vault`, `syscash_to_user`, `user_to_user`. Any other (including `org_to_user`) returns an error. | `service.go:planWallets` (426-508) | none |
| BR-TRF-11 | Fail-fast balance check: if `checkBalance` and sender balance < amount + fee, `ErrInsufficientBalance`. `syscash_to_user` skips it. | `service.go:339-341` | `TestEdge_E12_LockedUserFailsMidWorkflow` (indirect) |
| BR-TRF-12 | Fee is computed once at request time and stored on the row (`transactions.fee`); later policy changes do not alter it. | `service.go:334-337,357-367`; MOVE_FUNDS reads `txn.Fee` (`handlers.go:130-138`) | `TestP2PTransferEndToEnd` |
| BR-TRF-13 | The transaction row (status `processing`) and the workflow start are created in ONE DB transaction. | `service.go:350-398`; `queries/transactions.sql:InsertTransaction` | `TestP2PTransferEndToEnd` |
| BR-TRF-14 | MOVE_FUNDS legs: debit sender `amount+fee`, credit receiver `amount`, credit `SYSTEM_FEE` `fee` when fee > 0; compensation mirrors exactly. | `handlers/handlers.go:Wallet.legs` (124-141) | `TestP2PTransferEndToEnd`, `TestEdge_E07_CompensationReversesMovedFunds` |
| BR-TRF-15 | VALIDATE and MOVE_FUNDS re-check sender and receiver OWNER status (`active`) inside the workflow (E12) and VALIDATE re-checks balance >= amount+fee. | `handlers.go:Validator.Execute` (46-68), `Wallet.Execute` (97-106) | `TestEdge_E12_LockedUserFailsMidWorkflow` |
| BR-TRF-16 | CONFIRM completes the transaction only from `processing`. | `handlers.go:Confirmer`; `queries/transactions.sql:CompleteTransaction` | `TestP2PTransferEndToEnd` |
| BR-TRF-17 | Fee = `fixed + amount*percent_bp/10000` using integer division (truncation toward zero), then raised to `min_fee`, then capped to `max_fee` when set, then floored at 0. | `fee/fee.go:Calc` (25-37) | `TestCalc` |
| BR-TRF-18 | Policy selection: active policies of the txn type with `kyc_status = user tier OR NULL`; tier-specific wins over NULL; highest `version` wins; none found means fee 0. | `queries/fee.sql:GetFeePolicy`; `fee.go:For` (45-58) | none |
| BR-TRF-19 | Seed policy: `p2p` fixed 1000 VND, `topup` and `disbursement` 0, tier NULL. | `000004...up.sql:98-103` | `TestP2PTransferEndToEnd` (fee 1000 asserted on SYSTEM_FEE net, `transfer_test.go:44`) |
| BR-TRF-20 | OfferGuard resolves the user's offer via `user_offers` (offer must be active); with no assignment it falls back to the default offer code; no matching service row gives `ErrServiceNotInOffer`. | `offerguard.go:65-78`; `queries/offer_guard.sql` | `TestOfferGuard_UnknownServiceRefused` |
| BR-TRF-21 | `is_enabled=false` gives `ErrServiceDisabled`. | `offerguard.go:82-84` | `TestOfferGuard_ServiceDisabled` |
| BR-TRF-22 | `amount < min_amount` gives `ErrBelowMin`. | `offerguard.go:85-87` | `TestOfferGuard_BelowMinimum` |
| BR-TRF-23 | Per-transaction limit: `max_amount_per_txn > 0` and `amount > max` gives `ErrLimitPerTxn` (equal passes). | `offerguard.go:88-90` | `TestOfferGuard_LimitPerTxn` |
| BR-TRF-24 | Daily limit: `daily_limit > 0` and `used + amount > daily_limit` gives `ErrLimitDaily`. `used` = SUM(amount+fee) of the user's transactions of the same ledger txn type created since local midnight, excluding statuses `failed` and `reversed`. | `offerguard.go:93-102`; `queries/offer_guard.sql:SumUserTxnAmountSince` | none |
| BR-TRF-25 | Monthly limit: same, since the first of the month at 00:00 local. | `offerguard.go:103-112` | none |
| BR-TRF-26 | Interval throttle: `interval_seconds > 0` and last transaction of that type (any status) younger than the interval gives `ErrTooFrequent`. | `offerguard.go:113-121`; `queries/offer_guard.sql:LastUserTxnAt` | none |
| BR-TRF-27 | Quota is counted per ledger txn type, not per service: `P2P_TRANSFER` and `MONEY_REQUEST` share type `p2p` and therefore one counter. | `offerguard.go:126-132` (key is `txnType`); 000006:21-22 | none |
| BR-TRF-28 | Required auth is the matrix row covering the amount (`min_amount <= amount < max_amount`, NULL max means open); above all ceilings the highest tier; no rows or query error yields `pin`. The server only returns it as `auth_required`; it is not enforced (see 10). | `offerguard.go:authFor` (136-152); `service.go:319,403-413` | `TestAuthMatrix_FromDB` |
| BR-TRF-29 | Seeded default matrix: pin 0-2,000,000; otp 2,000,000-50,000,000; 2fa >= 50,000,000. Seeded offer_service for `OFFER_CUST_L2` x P2P: per txn 100,000,000, daily 500,000,000, monthly 2,000,000,000, interval 3 s; other active services in L1, L2, BIZ_STARTER: 0 / 100M / 500M / 2,000M / 0 s. | `000004...up.sql:85-95`; `000006...up.sql:106-124` | none |
| BR-TRF-30 | Vault moves require a vault the caller owns (`extra_params.vault_id`, `direction`). | `service.go:planWallets` (428-459) | `TestVault_ForeignVaultRejected`, `TestVault_DirectionMustBeValid` |
| BR-TRF-31 | Failure code shown in timelines is an allowlist: insufficient balance, "blocked by fraud", "not active", else `ERR_TRANSFER_FAILED`. | `service.go:failureCode` (195-206) | `TestTimeline_FailureCode` |

## 4. Lifecycle / state machine
Transaction status enum: `created, pending_approval, processing, processing_async, completed, failed, reversed` (000002:11). Transfer-created transactions start in `processing`.

```mermaid
stateDiagram-v2
    [*] --> processing: ExecuteEngineRequest (InsertTransaction)
    processing --> completed: CONFIRM (CompleteTransaction)
    processing --> failed: workflow failure (FailTransactionGuarded)
    completed --> reversed: reversal (MarkTransactionReversed)
```

| Transition | Owner |
|------------|-------|
| `-> processing` | `transfer/service.go:ExecuteEngineRequest`, SQL `InsertTransaction` (status literal `'processing'`) |
| `processing -> completed` | `handlers/handlers.go:Confirmer` via `CompleteTransaction` (`WHERE status='processing'`) |
| `-> failed` | workflow engine via `queries/workflow.sql:FailTransactionGuarded` (from created/processing/pending_approval/processing_async) |
| `completed -> reversed` | reversal package, out of scope |

Workflow definition for p2p-type services: `p2p_transfer` v2 = VALIDATE, FRAUD_CHECK, MOVE_FUNDS (on_failure compensate), CONFIRM, ROUND_UP (000009:18-27). v1 is deactivated.

## 5. Main flows

```mermaid
sequenceDiagram
    participant M as Mobile
    participant A as API (RequireAuth + RequireIdempotencyKey)
    participant T as transfer.ExecuteEngineRequest
    participant G as OfferGuard
    participant F as fee.Service
    participant E as workflow.Engine
    M->>A: POST /v1/transactions/request {service_code, amount, receiver}
    A->>T: input + Idempotency-Key
    T->>T: load service, active? dispatch profile
    T->>G: Check(user, service, txnType, amount)
    G-->>T: auth method or ErrLimit*/ErrBelowMin/...
    T->>T: planWallets, GetUserStatusByID
    T->>F: For(txnType, kyc tier, amount)
    T->>T: balance >= amount+fee ?
    T->>T: BEGIN; INSERT transactions (processing)
    alt idempotency key already used
        T-->>A: stored transaction (no new row)
    else new
        T->>E: Start(def, txnID) in same tx
        T->>T: COMMIT
        T-->>A: {transaction_id, ref_code, fee, total, auth_required}
    end
    Note over E: async: VALIDATE, FRAUD_CHECK, MOVE_FUNDS, CONFIRM, ROUND_UP
    M->>A: POST /transactions/confirm {transaction_id, pin}
    A->>T: ExecuteEngineConfirm
    T-->>A: current status only (no auth, no state change)
```

Key point: the "request / confirm / verify" trio is not a real 3-step flow. Money starts moving when `engine.Start` commits at the request step. `ExecuteEngineConfirm` only reads the row by id and returns ref_code, status, amount, fee (`service.go:515-528`); `ExecuteEngineVerify` calls it with the code as OTP (530-536). PIN/OTP are never verified.

## 6. Invariants
- Fee on the row equals what MOVE_FUNDS posts: legs are derived from the immutable row (BR-TRF-14).
- Sender debit = receiver credit + fee for p2p and vault moves, via the ledger balance rule.
- The transaction row and workflow start commit atomically (BR-TRF-13); a failed `engine.Start` rolls back the row.
- A request over a limit never creates a row (OfferGuard runs before the DB write). Counterpart caveat: limits are check-then-insert (BE-H2).

## 7. Data
Link: `../reference/data-model.md`.

| Table | Purpose |
|-------|---------|
| `transactions` | one row per request: `ref_code UNIQUE`, `type`, wallets, `amount > 0`, `fee >= 0`, `status`, `initiated_by`, `idempotency_key UNIQUE` |
| `services` | catalog plus dispatch columns |
| `offers`, `offer_services`, `offer_auth_matrix`, `user_offers` | Offer Factory limits (`user_offers` is one row per user, `ON CONFLICT (user_id)` in `AssignUserOffer`) |
| `fee_policies` | fee rules: `txn_type`, `kyc_status` (NULL = any), `fixed_fee`, `percent_bp`, `min_fee`, `max_fee`, `is_active`, `version` |

## 8. Interfaces
Mounted (`api/router.go`): `POST /v1/transactions/request` (118, idempotency required), `POST /v1/transactions/confirm` (119), `POST /v1/transactions/verify` (120), `GET /v1/fees/preview?amount=` (114, uses txn type `p2p` and the user KYC tier only, ignores service and offer), `GET /v1/transactions/:refCode/timeline` (113).
Dead code (not mounted): `TransferHandler.Transfer` (`api/transfer_handler.go:32`, no route references it), `transfer.Service.Transfer`, `TransferToUser`, `wallet.WalletHandler.Topup`. These legacy methods apply `wallet.TxnLimit` but not OfferGuard (consistent with audit downgrade of BE-M5 to Low).
Error mapping: limits -> `ERR_LIMIT_PER_TXN|ERR_LIMIT_DAILY|ERR_LIMIT_MONTHLY` (400), `ERR_TOO_FREQUENT` (429), `ERR_SERVICE_NOT_IN_OFFER` (403), `ERR_AMOUNT_BELOW_MIN` (400) (`api/errmap.go:58-64`). Link `../reference/api.md`.

## 9. Edge cases
- E01 idempotency: same key returns first transaction (BR-TRF-03); note payload is not compared.
- E07 compensation: if a step after MOVE_FUNDS fails, forward legs are mirrored, ledger nets to 0 (`TestEdge_E07_CompensationReversesMovedFunds`).
- E12 account locked mid-workflow: workflow fails before money moves and no entries are written (`TestEdge_E12_LockedUserFailsMidWorkflow`).
- No policy row for a txn type/tier: fee 0 (BR-TRF-18).
- Unknown `service_code`: silently treated as `P2P_TRANSFER` (BR-TRF-04); the response `service_code` shows the fallback.
- Interval throttle uses the latest attempt of any status, so a failed attempt still delays the next one within the interval.
- When no `Idempotency-Key` reaches the service (only possible outside the mounted route) a random UUID is used, so no dedupe (`service.go:344-347`).

## 10. Known gaps (as-is vs intended)
- BE-C2: confirm/verify do not verify PIN/OTP, do not check that the transaction belongs to the caller (IDOR: any transaction id returns its ref_code, amount, fee, status), and swallow DB errors into an untyped "not found" string (`service.go:515-528`). `auth_required` is advisory only. The old design (3-step request/confirm/verify with auth gate) is NOT what runs.
- BE-C1: MOVE_FUNDS can post twice on redelivery/crash (`handlers.go:83-121`; see ledger.md).
- BE-H1: idempotency key is global, not per user, and payload is not matched; a same-key request by another user returns the first user's result (`service.go:368-386`).
- BE-H2: OfferGuard TOCTOU: SUM then INSERT are not atomic, so parallel requests with different keys can pass limits and the interval throttle (`offerguard.go:92-121`, `service.go:315`). Day/month boundaries use the server's local timezone (`offerguard.go:94,104`), not a fixed Vietnam zone.
- BE-H4: compensation fails terminally if the receiver already spent the funds (`handlers.go:113-115`).
- Money request status is set before MOVE_FUNDS runs (BE-H5), documented in the money request domain.
- `wallet.TxnLimit` (KYC tier) is not applied on the mounted engine path; OfferGuard seed limits (100M per txn) are above the `none` tier limit of 2M, so an unverified user can send up to the offer's limit (not verified end to end).
- `GET /fees/preview` does not reflect the service or ledger type used by the engine request, so it can disagree with the fee actually frozen for non-p2p services.
- Fee truncates (rounds down) with no rounding unit; `amount*percent_bp` can overflow int64 only for absurdly large amounts (no guard).
- OfferGuard with `OFFERGUARD_ENABLED=false` removes all limits; no startup guard prevents that in production.
- `GetServiceByCode` failure fallback hides typos (BR-TRF-04).
- Error strings in `transfer/service.go` (lines 297, 486, 507, 518) and `offerguard.go` are Vietnamese; only the offerguard sentinels have `errTable` entries (`errmap.go`). The "dịch vụ hiện đang bảo trì" and "không tìm thấy giao dịch" errors are not mapped (audit mentions the latter).
- Dead legacy paths remain in code (`Transfer`, `TransferToUser`, `Topup`) and bypass OfferGuard if re-mounted.

## 11. Code map
| Path | Responsibility |
|------|----------------|
| `internal/transfer/service.go:277` | `ExecuteEngineRequest` main entry |
| `internal/transfer/service.go:426` | `planWallets` wallet strategies |
| `internal/transfer/service.go:515,530` | `ExecuteEngineConfirm`, `ExecuteEngineVerify` (status echo) |
| `internal/transfer/service.go:88,163,209` | `TransferToUser` (dead), `FeePreview`, `TimelineFor` |
| `internal/fee/fee.go:25,45` | `Calc`, `For` |
| `internal/offerguard/offerguard.go:64,136` | `Check`, `authFor` |
| `internal/workflow/handlers/handlers.go:37,82,149` | VALIDATE, MOVE_FUNDS, CONFIRM |
| `internal/api/transaction_engine_handler.go` | HTTP adapter for request/confirm/verify |
| `internal/product/service.go` | admin listing/CRUD of services, offers, orgs, audit; no money logic |
| `test/integration/transfer_test.go`, `transaction_request_http_test.go`, `vault_test.go`, `internal/offerguard/offerguard_test.go`, `internal/fee/fee_test.go` | tests |

## 12. Open questions
- Should confirm/verify enforce PIN/OTP before the workflow starts (owner: product/security, BE-C2, undecided)?
- Should quota be keyed per service rather than per ledger type (owner: product, undecided)?
- Which timezone defines "day" and "month" for limits (owner: product, undecided)?
- Should fee rounding use a unit (e.g. round to 1 VND is implicit) or ceil instead of floor (owner: finance, undecided)?
