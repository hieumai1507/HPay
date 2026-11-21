---
title: ADR 0007 KYC limits attached to offers
type: decision
status: accepted-not-implemented
last_verified: 2026-11-21
---

# ADR 0007: Limits are attached to the user's offer, selected by KYC tier

Status: Accepted (user decision, 2026-11-21). Not yet implemented.

## Context
Limits live in two unrelated mechanisms today (verified in code 2026-11-21):
- `wallet.TxnLimit(kycStatus)` in `internal/wallet/service.go` hard-codes 2,000,000 (none, rejected), 5,000,000 (pending, manual review) and 100,000,000 (verified, auto-approved). It is applied at `wallet/service.go:92`, `moneyrequest/service.go:208` (paying a request), the legacy `transfer/service.go:96` (`TransferToUser`) and is exposed as `txn_limit` by `api/user_handler.go:70`. The unified engine path (`ExecuteEngineRequest`) does NOT call it.
- The offer model (`offers`, `offer_services`, `offer_auth_matrix`, `user_offers`) enforced by `internal/offerguard` before a transaction enters the workflow. It defines per-service minimum, per-transaction, daily and monthly limits, a velocity interval, and the auth method per amount tier.

The two are not connected to KYC:
- `OFFER_CUST_L1` (`primary_pocket_max` 50,000,000) and `OFFER_CUST_L2` (200,000,000) are seeded in migration `000004`, but nothing links a KYC status to an offer. `AssignUserOffer` (sqlc query) is called only from `offerguard_test.go`; the seeder assigns offers with raw SQL.
- `user_offers` is backfilled in `000006` only to `OFFER_CUST_L2` (and Biz Starter for the demo org phones). New users get no row and `offerguard` falls back to `DefaultOfferCode = "OFFER_CUST_L2"` (override `DEFAULT_OFFER_CODE`).
- The two customer offers have identical service limits: `000006` step 5 inserted the same rows for `OFFER_CUST_L1` and `OFFER_CUST_L2` (per transaction 100,000,000, daily 500,000,000, monthly 2,000,000,000; interval 0 except the seeded L2 P2P row with 3 s). Only the auth matrix and seed rows differ in places. So tier 1 is not more restricted than tier 2 on the engine path. (The lead's brief said L1 has no `offer_services` rows; the local database and migration `000006` show 5 rows for L1, so that part is corrected here.)
- `primary_pocket_max` is stored but no code enforces it.
- An inactive offer does not block: `GetOfferServiceForUser` joins `offers ... AND o.is_active`, so a deactivated assigned offer returns no row, and `offerguard` then falls back to the default offer instead of refusing.
- Documents and mockups disagree on the numbers (audit section 4, item 2).

## Decision
1. A user's limits and auth method come from their OFFER, never from code constants: the offer defines per-transaction, daily and monthly limits and the auth method (`offers` + `offer_services` + `offer_auth_matrix`).
2. The user's KYC tier selects the offer. KYC status changes assign the matching offer.
3. The hard-coded `wallet.TxnLimit` is removed once the offer path covers every money-moving flow (engine, money request payment, wallet topup, legacy transfer).

## Consequences
- One authority for limits, configurable from the admin offer screens, testable per offer.
- Follow-ups required before `TxnLimit` can be deleted:
  - Product question, still open: the tier-to-offer mapping and the final limit table. No numbers are decided here; the figures above describe current code only.
  - Assign the offer in the same transaction as the KYC status change (kyc classifier and admin review); assign an offer at registration so the fallback is not the normal path.
  - Give the tier offers distinct `offer_services` rows (L1 must be stricter than L2).
  - Make an inactive or missing assigned offer block (fail closed) instead of falling back to the default.
  - Enforce `primary_pocket_max` (maximum balance) or drop the column.
  - Move every `TxnLimit` call site to the offer guard, update `txn_limit` in `/users/me` and `TestEdge_E13_LimitByKycTier`, then delete `TxnLimit`.
- Related open findings: offer-limit TOCTOU race (BE-H2), server-time-zone day boundaries, `wallet.TxnLimit` not applied on the engine path (audit Medium).