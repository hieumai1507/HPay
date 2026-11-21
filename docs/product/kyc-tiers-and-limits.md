---
title: KYC tiers and limits
type: product
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/db/migrations/000004_offer_factory_policies.up.sql, src/backend/db/migrations/000006_service_dispatch_user_offers.up.sql, src/backend/internal/wallet/service.go, src/backend/internal/offerguard/offerguard.go, src/backend/internal/store/queries/offer_guard.sql, src/backend/internal/org/service.go, src/backend/internal/kyc/classify.go, src/backend/internal/fee/fee.go]
---

# KYC tiers and limits

## Product decision (2026-11-21)

**Limits follow the offer.** Limits live in the offer model (`offers`, `offer_services`, `offer_auth_matrix`). A user's KYC tier is meant to select the offer, and the offer defines the limits. **The tier-to-offer mapping and the final limit table are not yet defined.** This page does not state a recommended or final limit table. The open table is in `docs/planning/PLAN.md`, section "Decision update 2026-11-21".

Labels used below: **seed** = value inserted by a migration (a starting data row, not a rule); **code** = value hard-coded in Go; **undecided** = no value exists and a product decision is pending. Nothing labeled seed or code here is a decided product rule.

This replaces the earlier D7 note in PLAN.md section 5.2 ("take the seed values in code, L1 = 50M, L2 = 200M"), which the 2026-11-21 decision supersedes.

## How limits are computed today

### A. Offer limits (OfferGuard), applied on the mounted engine path

`POST /v1/transactions/request` calls `offerguard.Check(user, service, txnType, amount)` (`transfer/service.go:311-320`). Resolution (`offerguard.go:62-80`, `store/queries/offer_guard.sql`):

1. Look up the user's `user_offers` row joined to an **active** offer and its `offer_services` row for the service.
2. If no row is found, fall back to the default offer `OFFER_CUST_L2` (`DefaultOfferCode`, env `DEFAULT_OFFER_CODE`, `offerguard.go:44-59`) for that service.
3. If still none: `ErrServiceNotInOffer` (403).
4. Then checks in order: service enabled, `min_amount`, `max_amount_per_txn`, daily volume (`SUM(amount+fee)` since start of day, failed/reversed excluded), monthly volume, `interval_seconds` throttle. The result also carries the required auth method from `offer_auth_matrix` (only a label, nothing verifies it for user transactions).

Because step 2 triggers whenever the user's own offer yields no row, **an inactive offer does not block; it silently falls back to the default offer.**

### B. `wallet.TxnLimit` (code), a per-transaction cap by KYC status

`wallet/service.go:33-42`, constants in Go:

| `kyc_status` | Per transaction (code) |
|---|---|
| `none`, `rejected` | 2,000,000 |
| `pending`, `manual_review` | 5,000,000 |
| `auto_approved`, `verified` | 100,000,000 |

It is the current cap by KYC status but it is **not applied on the mounted engine path** (`ExecuteEngineRequest` never calls it). It is still used by: money-request pay (`moneyrequest/service.go:208`), and by `GET /v1/users/me` which returns it as `txn_limit` (`api/user_handler.go:70`; the mobile top-up screen compares against that value). Its other callers, `transfer.TransferToUser` and `wallet.Topup`, are reachable only through handlers that are not mounted. Per the decision it is not the source of truth and is to be removed once the offer path covers every money flow. KYC status values are set by the KYC workflow: score >= 85 `auto_approved`, 50-84 `manual_review`, < 50 `rejected` (`kyc/classify.go`), then an admin may set `verified` or `rejected`.

## Offer data in the migrations (seed)

Offers seeded in migration 000004:76-82:

| Offer code | Level | Role | `primary_pocket_max` (seed) | Max bank accounts (seed) |
|---|---|---|---|---|
| `OFFER_CUST_L1` | 1 | customer | 50,000,000 | 2 |
| `OFFER_CUST_L2` | 2 | customer | 200,000,000 | 5 |
| `OFFER_BIZ_STARTER` | 3 | merchant | 1,000,000,000 | 10 |
| `OFFER_BIZ_ENT` | 5 | merchant | 5,000,000,000 | 20 |

Service limits seeded in 000004:85-95: exactly one `offer_services` row, `OFFER_CUST_L2` x `P2P_TRANSFER`: min 0, per transaction 100,000,000, daily 500,000,000, monthly 2,000,000,000, interval 3 s (all seed). Auth matrix for that row: PIN below 2,000,000, OTP 2,000,000 to 50,000,000, 2FA from 50,000,000 (seed; only a label, nothing verifies it).

Migration 000006:103-124 (a later migration, so these rows also exist on a fully migrated database) inserts for every active service x (`OFFER_CUST_L1`, `OFFER_CUST_L2`, `OFFER_BIZ_STARTER`) a row with min 0, per transaction 100,000,000, daily 500,000,000, monthly 2,000,000,000, interval 0 (seed, `ON CONFLICT DO NOTHING` so the L2 x P2P row from 000004 wins), and the same PIN / OTP / 2FA matrix for any row without one. The text of `docs/planning/PLAN.md` says `OFFER_CUST_L1` has no `offer_services` rows; that is true after 000004 alone but not after 000006 (see Known gaps). `OFFER_BIZ_ENT` has no `offer_services` rows in either migration. These 000006 numbers are placeholder defaults for demo flows ("sane default limits", comment at 000006:103-104), not product decisions.

Assignment of users to offers: `user_offers` is backfilled to `OFFER_CUST_L2` for every existing user, and the four seeded org-demo phones to `OFFER_BIZ_STARTER` (000006:27-37). `AssignUserOffer` (`store/queries/offer_guard.sql:37`) is called only from a test (`offerguard_test.go:61`) and from the demo seeder (`cmd/seed/main.go:90-94`). Registration and KYC approval do not create or change a `user_offers` row, so new users fall through to the default offer `OFFER_CUST_L2`.

### Other numbers present in code

| Item | Value | Label | Where |
|---|---|---|---|
| Org disbursement max per transaction | 1,000,000,000 | code | `org/service.go:46,285,369` |
| Money request validity | 72 h | code | `moneyrequest/service.go:22` |
| Approval request validity | 24 h default | code (step config `expires_h`) | `approval/service.go:124-129` |
| OTP | TTL 120 s, 3 attempts, 30 s cooldown | code | `auth/otp.go:15-17` |
| Fraud flag / block | flag at score >= 40, block above 70 | code | `fraud/rules.go` |
| P2P fee | fixed 1,000 VND; topup 0; `disbursement` 0 | seed (000004:98-103) | `fee_policies` |

Fee = fixed + amount x percent_bp / 10,000, rounded down; tier-specific row beats the default; **no row means fee 0** (`fee/fee.go:44-50`). No tier-specific fee rows are seeded. The seed uses txn type `disbursement` but the org service looks up `org_disbursement`, so org disbursement currently pays no fee.

## Limit table

| Offer | KYC tier mapped to it | Per transaction | Per day | Per month | Wallet balance ceiling (`primary_pocket_max`) |
|---|---|---|---|---|---|
| (unverified user) | undecided | undecided | undecided | undecided | undecided |
| `OFFER_CUST_L1` | undecided | 100,000,000 (seed, placeholder from 000006, all services) | 500,000,000 (seed, placeholder) | 2,000,000,000 (seed, placeholder) | 50,000,000 (seed) |
| `OFFER_CUST_L2` | undecided | 100,000,000 (seed, P2P and placeholder for other services) | 500,000,000 (seed) | 2,000,000,000 (seed) | 200,000,000 (seed) |

Nothing in this table is a decided rule; every "undecided" cell needs a product decision and the "seed" cells are only what the migrations put in the database. The separate code cap `TxnLimit` (2M / 5M / 100M by KYC status) is described above and is not part of the offer table.

## Known gaps (code versus intent)

1. **Tier is not linked to offer.** Nothing maps KYC status to an offer. KYC approval does not touch `user_offers`; only a test and the demo seeder call `AssignUserOffer`; 000006 backfills everyone to `OFFER_CUST_L2`; new users fall back to the default `OFFER_CUST_L2`. So today every personal user is limited by the same offer regardless of KYC status.
2. **`OFFER_CUST_L1` data.** Migration 000004 seeds no `offer_services` rows for L1, but 000006:106-112 inserts L1 x every active service with placeholder limits identical to L2 (100M per transaction, 500M per day, 2B per month). On a fully migrated database L1 is therefore not blocked, and its limits equal L2's. A service added after 000006 would have no L1 row until one is inserted. Not yet confirmed on a real database.
3. **Only L2 x P2P is a deliberate seed** (000004). All other `offer_services` rows come from 000006's blanket defaults. `OFFER_BIZ_ENT` has none.
4. **Inactive offer falls back to the default offer instead of blocking** (`GetOfferServiceForUser` joins `o.is_active`; no row triggers the fallback at `offerguard.go:68-80`). Intent: block.
5. **`wallet.TxnLimit` is not applied on the engine path** and is only live in money-request pay and `/users/me`. It disagrees with the offer numbers (100M per transaction for verified, 2M for unverified, versus the offer's 100M for everyone). To be removed after the offer path covers every money flow.
6. **`primary_pocket_max` is not enforced anywhere.** It is a wallet balance ceiling, not a per-transaction limit. It is only stored and returned by the admin offer pages (`product/service.go:131,144`); no code checks a wallet balance or credit against it. Intent: enforce on credits or remove it from the offer model.
7. **Auth matrix is a label only.** `auth_required` is returned but no PIN/OTP/2FA is verified for user transactions (audit BE-C2).
8. **Daily/monthly checks use the server timezone and are not locked** (read then insert, audit BE-H2); the window start uses `time.Now().Location()`, UTC in the distroless image.
9. **Admin cannot edit offers.** `product/service.go` has the queries but no write route is mounted; the admin offer editor posts to nonexistent endpoints, so offers can only be changed by migration or SQL.
10. **User-facing and design text states other numbers.** None of these are decisions; they need aligning once the table is decided:
    - `src/mobile/app/(kyc)/intro.tsx:39` ("100 triệu ₫/ngày", shown to users)
    - `docs/design/mockups/01-auth-kyc.html:118,185`, `03-business.html:20`, `07-admin-offer-service.html:119-121`
    - `docs/history/plans/backend/f3-ledger-wallets/implementation-plan.md:52`, `f8-kyc/implementation-plan.md:48`, `f8-kyc/solution-architecture.md:65`, `plans/mobile/m2-auth-kyc/implementation-plan.md:25`
    - `docs/history/design-v2/05-offer-service-factory.md:79-81`
    - `src/admin/src/pages/OfferDetailPage.tsx:15-23,67-91` (editor defaults, local constants)
    - `docs/planning/PLAN.md` section 5.2 (superseded by the 2026-11-21 decision)
    - `docs/history/guide-v2/functional-overview.md:268-281` quotes 5,000,000 VND, which is an org approval-tier example, not a KYC limit.
