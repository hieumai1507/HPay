---
title: Offer limits and KYC tier to offer mapping
type: spec
status: approved-numbers
last_verified: 2026-11-21
adr: ../../adr/0007-kyc-limits-attached-to-offers.md
numbers: ../offer-limits/benchmark-and-proposal.md
audit: [BE-M5]
---

# Spec: limits follow the offer, the KYC tier selects the offer

User decisions (2026-11-21): limits are attached to offers (ADR-0007); the T0/T1/T2 table in `offer-limits/benchmark-and-proposal.md` is approved; the working assumptions for the open questions stand (single currency, no weekly window yet).

## Scope of this slice
1. Migration 000023: offer `OFFER_CUST_L0`; per-service limits for L0, L1, L2 from the approved table converted to VND at **25,000 VND per USD** (assumption, change in one migration if the rate changes); balance ceiling values stored in `offers.primary_pocket_max` (stored only, not enforced yet).
2. KYC outcome selects the offer in the same transaction as the status change: auto-approved or admin-approved moves L0 to L1 (L1 and L2 are kept); rejected moves any customer offer to L0. Manual review and pending change nothing.
3. A user with no `user_offers` row falls back to `OFFER_CUST_L0` (most restrictive). A user who has a row is never sent to the default offer.
4. An inactive offer blocks (new error `ErrOfferInactive`, 403 `ERR_OFFER_INACTIVE`); it does not fall back.
5. `wallet.TxnLimit` is removed. Every path that used it asks OfferGuard instead (`/users/me`, money-request pay, and the unmounted legacy `wallet.Topup`, `transfer.Transfer`, `transfer.TransferToUser`).
6. Existing users whose offer is the system-assigned `OFFER_CUST_L2` backfill are remapped by KYC status.

## Limits (VND, 25,000 per USD)
| Offer | Per transaction | Per day | Per month | Balance ceiling (stored) |
|---|---|---|---|---|
| OFFER_CUST_L0 | 2,500,000 | 2,500,000 | 25,000,000 | 6,250,000 |
| OFFER_CUST_L1 | 62,500,000 | 125,000,000 | 625,000,000 | 250,000,000 |
| OFFER_CUST_L2 | 250,000,000 | 375,000,000 | 2,500,000,000 | 1,250,000,000 |

Weekly (rolling) limits from the table are NOT implemented: they need a window feature of their own (separate slice, ADR). Until then daily and monthly apply. The auth matrix (PIN below 2M, OTP 2M to 50M, 2FA above) is copied unchanged; the 2FA band is still refused by the server (ADR-0014), so amounts above 50M need the 2FA provider that does not exist yet.

## Acceptance criteria
- AC-1 L0 per-transaction cap blocks an amount above 2,500,000 with `ERR_LIMIT_PER_TXN`; L1 allows it up to 62,500,000.
- AC-2 A user with no offer row is limited as L0.
- AC-3 KYC auto-approval moves L0 to L1 atomically; a user already at L2 stays at L2; admin approval does the same.
- AC-4 KYC rejection moves L1 or L2 to L0; a merchant offer is never touched.
- AC-5 An inactive offer blocks with `ERR_OFFER_INACTIVE`, and a user who has an offer row but no row for the service gets `ERR_SERVICE_NOT_IN_OFFER`, not the default offer.
- AC-6 `GET /v1/users/me` returns `txn_limit` equal to the per-transaction limit of the user's offer for P2P_TRANSFER, plus the offer code and the three limits.
- AC-7 Money-request pay, wallet top-up and legacy transfer respect the offer limits.
- AC-8 Migration up then down then up is clean; existing system-assigned L2 users are remapped by KYC status.

## Not in this slice
Weekly windows, currency model, balance-ceiling enforcement, per-tier auth thresholds, compliance review per jurisdiction, T2 qualification (admin assigns L2 by hand today).