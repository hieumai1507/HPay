---
title: KYC tiers and limits
type: product
status: verified-against-code
last_verified: 2026-11-21
c
---

# KYC tiers and limits

## Product decision (2026-11-21)

**Limits follow the offer** (ADR-0007) and **the KYC outcome selects the offer**. The numbers are the T0/T1/T2 table approved by the owner on 2026-11-21 (`specs/offer-limits/benchmark-and-proposal.md`), which is a US-dollar table (an earlier version stored it multiplied by 25,000 as Vietnamese dong; ADR-0019 returned it to dollars, stored as cents). Spec: `specs/w3-offer-limits/spec.md`. Implemented in the working tree (migration 000023), not yet committed. Compliance review per jurisdiction is still required before launch.

## How limits are computed

### Offer limits (OfferGuard)

`offerguard.Guard` resolves limits for (user, service):

1. A user **with** a `user_offers` row is judged by that offer only. A missing `offer_services` row is `ErrServiceNotInOffer` (403) and an inactive offer is `ErrOfferInactive` (403 `ERR_OFFER_INACTIVE`); neither falls back to another offer.
2. A user with **no** row uses the default offer `OFFER_CUST_L0` (`DefaultOfferCode`, env `DEFAULT_OFFER_CODE`), the most restrictive one.
3. `Check` (engine path) then applies: service enabled, `min_amount`, `max_amount_per_txn`, daily volume, monthly volume, `interval_seconds` throttle. `CheckAmount` (legacy paths: money-request pay, `wallet.Topup`, `transfer.Transfer*`) applies the offer, enabled flag, minimum and per-transaction cap. `Limits` feeds `GET /v1/users/me` (`txn_limit` and an `offer` block).

### KYC outcome selects the offer (`kyc/offer.go`)

Applied in the same transaction as the status change, by the workflow CLASSIFY step and by the admin review:

| New KYC status | Offer change |
|---|---|
| `verified`, `auto_approved` | `OFFER_CUST_L0` becomes `OFFER_CUST_L1`; L1, L2 and merchant offers are kept |
| `rejected` | any customer offer becomes `OFFER_CUST_L0`; merchant offers are never touched |
| `pending`, `manual_review` | no change |

`OFFER_CUST_L2` is assigned by hand (the "fully verified" qualification is not defined yet). Migration 000023 remapped users still on the system-assigned L2 backfill: verified or auto-approved to L1, everyone else to L0.

`wallet.TxnLimit` no longer exists.

## Limit table (US dollars; stored as cents, 1 = $0.01)

| Offer | How a user gets it | Per transaction | Per day | Per month | Balance ceiling (`primary_pocket_max`, stored only) |
|---|---|---|---|---|---|
| `OFFER_CUST_L0` | no KYC, rejected KYC, or no offer row | $100 | $100 | $1,000 | $250 |
| `OFFER_CUST_L1` | KYC verified or auto-approved | $2,500 | $5,000 | $25,000 | $10,000 |
| `OFFER_CUST_L2` | assigned by an operator | $10,000 | $15,000 | $100,000 | $50,000 |
| `OFFER_BIZ_STARTER` | org demo actors | $4,000 (placeholder) | $20,000 | $80,000 | $40,000 |
| `OFFER_BIZ_ENT` | none assigned | no service rows | | | $200,000 |

Same limits for every service in the offer (the approved table is per user, not per service). The auth matrix is unchanged: PIN below $80, OTP from $80 to $2,000, 2FA above (refused by the server today, ADR-0014).

Not implemented from the approved table: **weekly rolling limits** (the offer model has daily and monthly only; needs its own slice and ADR), balance-ceiling enforcement, multi-currency (ADR-0019 fixes one currency), and per-tier auth thresholds.

## Known gaps (code versus intent)

1. **Tier to offer: done** (KYC approval and rejection move the offer, see above). Still open: nothing assigns `OFFER_CUST_L2` automatically, and registration creates no `user_offers` row (the L0 default applies).
2. **`OFFER_CUST_L1` data.** Migration 000004 seeds no `offer_services` rows for L1, but 000006:106-112 inserts L1 x every active service with placeholder limits identical to L2 (the placeholder business limits: $4,000 per transaction, $20,000 per day, $80,000 per month). On a fully migrated database L1 is therefore not blocked, and its limits equal L2's. A service added after 000006 would have no L1 row until one is inserted. Not yet confirmed on a real database.
3. **Only L2 x P2P is a deliberate seed** (000004). All other `offer_services` rows come from 000006's blanket defaults. `OFFER_BIZ_ENT` has none.
4. **Inactive offer blocks: done** (`ErrOfferInactive`).
5. **`wallet.TxnLimit` is not applied on the engine path** and is only live in money-request pay and `/users/me`. It disagrees with the offer numbers (100M per transaction for verified, 2M for unverified, versus the offer's 100M for everyone). To be removed after the offer path covers every money flow.
6. **`primary_pocket_max` is not enforced anywhere.** It is a wallet balance ceiling, not a per-transaction limit. It is only stored and returned by the admin offer pages (`product/service.go:131,144`); no code checks a wallet balance or credit against it. Intent: enforce on credits or remove it from the offer model.
7. **Auth matrix is a label only.** `auth_required` is returned but no PIN/OTP/2FA is verified for user transactions (audit BE-C2).
8. **Daily/monthly checks** now use Asia/Ho_Chi_Minh and are re-checked at confirm under a per-user lock (BUG-0017). A request that needs no confirmation is still checked only at request time.
9. **Admin cannot edit offers.** `product/service.go` has the queries but no write route is mounted; the admin offer editor posts to nonexistent endpoints, so offers can only be changed by migration or SQL.
10. **User-facing and design text states other numbers.** None of these are decisions; they need aligning once the table is decided:
    - `src/mobile/app/(kyc)/intro.tsx:39` (a daily-limit sentence shown to users; the mobile agent of w6-usd rewrites it in dollars)
    - `docs/design/mockups/01-auth-kyc.html:118,185`, `03-business.html:20`, `07-admin-offer-service.html:119-121`
    - `docs/history/plans/backend/f3-ledger-wallets/implementation-plan.md:52`, `f8-kyc/implementation-plan.md:48`, `f8-kyc/solution-architecture.md:65`, `plans/mobile/m2-auth-kyc/implementation-plan.md:25`
    - `docs/history/design-v2/05-offer-service-factory.md:79-81`
    - `src/admin/src/pages/OfferDetailPage.tsx:15-23,67-91` (editor defaults, local constants)
    - `docs/planning/PLAN.md` section 5.2 (superseded by the 2026-11-21 decision)
    - `docs/history/guide-v2/functional-overview.md:268-281` quotes 5,000,000 VND, which is an org approval-tier example, not a KYC limit.
