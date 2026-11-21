---
title: Offer limits - market benchmark and proposal
type: spec
status: proposed          # NOT decided. Needs the user's approval and a compliance review per jurisdiction.
last_verified: 2026-11-21
code:
  - src/backend/db/migrations/000004_offer_factory_policies.up.sql
  - src/backend/db/migrations/000006_service_dispatch_user_offers.up.sql
  - src/backend/internal/offerguard/offerguard.go
related:
  - ../../product/kyc-tiers-and-limits.md
  - ../../adr/0007-kyc-limits-attached-to-offers.md
---

# Offer limits: market benchmark and proposal

Decision already taken (ADR 0007): limits follow the user's offer, and the KYC tier selects the offer. This page proposes the **numbers and the tier-to-offer mapping**. Nothing here is implemented or approved.

## 1. What the market does (evidence and its quality)

All figures below come from third-party blogs found by web search on 2026-11-21, not from the providers' own pages. Several disagree with each other. Use them for the *shape* of limits, not as exact values, and confirm against each provider's help center before relying on a number.

| Provider | Unverified | Verified | Confidence |
|----------|------------|----------|------------|
| Cash App | about $250 sent per rolling week; about $1,000 received per 30 days | about $7,500 sent per week; receiving unlimited. Some sources instead say $1,000 / $40,000 per 30 days | medium (sources disagree) |
| Venmo | $299.99 per week to people and merchants; bank transfer up to $999.99 per week | newest sources: $60,000 per week; older sources: $4,999.99 to $7,000 per week | low for the verified figure |
| PayPal | roughly $500 to $1,000 receive; lifetime send cap of about $4,000 (sources conflict) | no overall send cap for most users; single payment often cited at $10,000 or $60,000 | low |
| EU e-money (AML regulation) | stored value of EUR 250 (older directive) with EUR 150 cited for the newer regulation; the EBA guidance allows EUR 500 for domestic-only products | n/a | EUR 250 is well sourced; EUR 150 comes from a secondary summary, check Regulation (EU) 2024/1624 itself |
| Revolut / Wise | no usable source found (results were sales listings or competitor blogs) | none | none |

Sources: [Cash App limits](https://wealthvieu.com/banking/cash-app/cash-app-limits/), [Venmo limits](https://eco.com/support/en/articles/17345351-venmo-limits-2026-daily-weekly-and-instant-transfer-caps), [PayPal limits](https://tipalti.com/blog/paypal-transfer-limit/), [EBA response on CDD thresholds](https://www.eba.europa.eu/eba-response/13821), [Regulation (EU) 2024/1624](https://eur-lex.europa.eu/legal-content/EN/TXT/HTML/?uri=CELEX%3A32024R1624).

What is consistent across providers:
1. **Three levels**: unverified (small, often weekly caps), verified (an order of magnitude or more higher), and a higher or business level.
2. **Rolling windows**: weekly or 30-day, not calendar day or month.
3. **Receiving is limited less than sending**, and unverified receiving is capped or held.
4. **A small cap on stored value for unverified wallets** (EUR 150 to 250 in EU rules).
5. **Larger single payments need stronger authentication** (this matches the existing PIN / OTP / 2FA matrix in the seed).

## 2. How the current system differs

- Offers support **per transaction, daily, monthly** limits and a minimum interval. There is **no weekly or rolling-window limit**, which is what the market uses. Closing this is a feature of its own (spec and ADR needed).
- The code is **VND only** (`wallets.currency` has no CHECK, `ledger_entries` carries no currency). "A general market" implies multiple currencies. That is an architecture question, not a limits question; see the open questions.
- Both customer offers currently have identical placeholder limits (100M / 500M / 2B VND), so a tier gives no differentiation today.

## 3. Proposal (USD-equivalent; local amounts to be derived per currency)

Three personal tiers plus the existing business offers. Every number is a proposal derived from the benchmark above, with the reasoning stated, so you can accept or change each one.

| Tier / offer | How a user gets it | Per transaction | Per day | Per week (rolling) | Per month (rolling) | Wallet balance ceiling | Reasoning |
|---|---|---|---|---|---|---|---|
| **T0 Unverified** (`OFFER_CUST_L0`, new) | account created, no KYC | $100 | $100 | $250 | $1,000 | $250 | Matches Cash App $250 and Venmo $299.99 weekly caps and the EU e-money stored-value cap |
| **T1 Basic verified** (`OFFER_CUST_L1`) | ID document + selfie passed (auto-approved or manually approved) | $2,500 | $5,000 | $7,500 | $25,000 | $10,000 | Anchored on Cash App's $7,500 verified weekly figure |
| **T2 Fully verified** (`OFFER_CUST_L2`) | enhanced checks (for example proof of address or source of funds) | $10,000 | $15,000 | $25,000 | $100,000 | $50,000 | Between Cash App ($7,500) and Venmo's newest figure ($60,000); $10,000 matches the single-payment cap often cited for PayPal |
| Business (`OFFER_BIZ_STARTER`, `OFFER_BIZ_ENT`) | org onboarding | keep seeds; `OFFER_BIZ_ENT` has no service rows at all today | | | | | Out of scope until the org model is reviewed |

Auth method by amount (keep the existing three-step shape, scale the thresholds to the tier): PIN below 2% of the tier's per-transaction limit... *not decided; the current seed uses PIN under 2M, OTP 2M to 50M, 2FA above 50M VND.* Open question below.

Rules that go with the table:
- **Rejected or expired KYC** returns the user to T0.
- **KYC upgrade** changes the offer in the same transaction as the KYC status change; a downgrade applies to new transactions only.
- **Inactive offer blocks** (it does not fall back to a default).
- Limits count completed and in-flight transactions, exclude failed and reversed ones, and use a fixed time zone for calendar windows.

## 4. Open questions for the user (nothing below is decided)

1. **Currency**: is the product single-currency per deployment (for example VND for Vietnam) or truly multi-currency? Multi-currency needs a currency column on the ledger and an exchange-rate source, which is a large architecture change. Until decided, state limits in the deployment currency and convert once with a published rate (the exchange rate is an assumption to confirm).
2. **Weekly windows**: add a rolling weekly (and rolling 30-day) limit to the offer model, or keep daily and monthly only?
3. **Tier definitions**: what exactly qualifies T1 and T2 (documents, data checks, manual review), and which jurisdictions' rules apply? **This needs compliance or legal review; the benchmark is not regulatory advice.**
4. **Balance ceilings**: enforce the wallet balance ceiling (the `primary_pocket_max` field exists but nothing enforces it)?
5. **Auth thresholds**: scale PIN / OTP / 2FA bands per tier, or keep fixed amounts?

## 5. If accepted

Goes through `hpay-feature`: a spec with acceptance criteria, an ADR if the weekly window or multi-currency is chosen, then migration for the offer rows and the tier-to-offer assignment, the guard changes, removal of `wallet.TxnLimit`, tests, and updates to `product/kyc-tiers-and-limits.md`.