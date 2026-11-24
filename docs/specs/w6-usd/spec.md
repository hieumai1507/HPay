---
title: US dollars stored as cents (implementation of ADR-0019)
type: spec
status: in-progress
last_verified: 2026-11-25
adr: ../../adr/0019-usd-currency-in-cents.md
---

# Spec: US dollars stored as cents

Stages 1 and 2 of `hpay-feature` are collapsed here: the owner answered the three open questions on 2026-11-25 by accepting the recommendations (unit, fee, limits). ADR-0019 holds the design.

## Problem
The product is presented as a US-dollar wallet but the system is built for Vietnamese dong: default currency `VND`, amounts and limits in dong, `₫` in every screen, screenshot and guide.

## Goals
1. One currency, `USD`, amounts in integer cents, shown as `$1,234.56`.
2. Limits return to the approved dollar table; the P2P fee is `$0.25`.
3. A single place defines the default currency in Go; the apps share one money module each.
4. Typed amounts accept up to two decimals; nothing uses floats.

## Non-goals
More than one currency, exchange rates, sub-cent amounts, a data migration for databases that already ran the old migrations (none exist).

## Conversion table (cents; old dong value divided by 250)
| Item | Old (VND) | New (cents) | Shown as |
|------|-----------|-------------|----------|
| L0 per txn / day / month | 2,500,000 / 2,500,000 / 25,000,000 | 10,000 / 10,000 / 100,000 | $100.00 / $100.00 / $1,000.00 |
| L1 per txn / day / month | 62,500,000 / 125,000,000 / 625,000,000 | 250,000 / 500,000 / 2,500,000 | $2,500 / $5,000 / $25,000 |
| L2 per txn / day / month | 250,000,000 / 375,000,000 / 2,500,000,000 | 1,000,000 / 1,500,000 / 10,000,000 | $10,000 / $15,000 / $100,000 |
| Balance ceiling L0 / L1 / L2 | 6,250,000 / 250,000,000 / 1,250,000,000 | 25,000 / 1,000,000 / 5,000,000 | $250 / $10,000 / $50,000 |
| Auth tiers PIN / OTP / 2FA | below 2,000,000 / to 50,000,000 / above | below 8,000 / to 200,000 / above | below $80 / to $2,000 / above |
| P2P fixed fee | 1,000 | 25 (decision) | $0.25 |
| Round-up unit | 10,000 | 100 | $1.00 |
| Biz starter per txn / day / month | 100,000,000 / 500,000,000 / 2,000,000,000 | 400,000 / 2,000,000 / 8,000,000 | $4,000 / $20,000 / $80,000 |
| Largest typed amount (apps) | 1,000,000,000 | 4,000,000 | $40,000.00 |

Other seeded demo amounts follow the same division, rounded to a whole cent, with demo values chosen to read well (decided in the plan, not business rules).

## Acceptance criteria
- **AC-1 Currency.** Given a new user, when the wallet is created, then its currency is `USD`; the three system wallets are `USD`; no Go code outside the constant writes `"VND"`.
- **AC-2 Display.** Given an amount of `123456` cents, when any admin or mobile screen shows it, then it reads `$1,234.56`; `5` reads `$0.05`; `0` reads `$0.00`; a missing value reads an em dash, never `$0.00`.
- **AC-3 Input.** Given the amount field, when the user types `12`, `12.5`, `12.50` or `1,234.56`, then the value is 1200, 1250, 1250, 123456 cents; `12.345`, `1.234.567`, `-1`, `1e3`, letters and an empty field are rejected with a message.
- **AC-4 Bounds.** Given the maximum, when the user types more than `$40,000.00` in the mobile app, then it is rejected (C-01 upper bound); the admin upper bound is 1,000,000,000,000 cents as before.
- **AC-5 Limits.** Given a verified user at L1, when they request more than `$2,500.00` in one payment, then the existing per-transaction rule refuses it; the daily and monthly rules refuse at `$5,000.00` and `$25,000.00` total; the same holds under parallel confirms (BUG-0017 tests, converted).
- **AC-6 Fees.** Given a P2P transfer of any amount, then the fee is `25` cents; given a policy of 25 basis points, `Calc(p, 99)` is 0 and `Calc(p, 10000)` is 25 (round down); min and max clamp as before.
- **AC-7 Split.** Given a bill of `$10.00` split three ways, then the shares are 334, 333, 333 cents and sum to 1000; a total smaller than the number of people is rejected.
- **AC-8 Round-up.** Given a payment of `$12.34` and a unit of 100, then the round-up is 66 cents; a payment of `$12.00` rounds up by nothing.
- **AC-9 Seed and migrations.** Given an empty database, when all migrations run and the seed runs, then every wallet balance equals its ledger sum and the seeded offers match the conversion table.
- **AC-10 Ledger integrity under load.** After the load test, zero wallets differ from their ledger sum.
- **AC-11 Docs and visuals.** The domain, reference and product pages, the OpenAPI text, the admin user guide with retaken screenshots, the site and the README show dollars, and the guide checker passes.

## Invariants touched
C-01 (integer minor units, input upper bounds, no `Number()` on amount text), C-08 (edited migrations keep their down files), C-12 (unchanged).

## API and schema delta
No endpoint or column changes. Default currency string and seeded values change; OpenAPI descriptions say "cents"; the env name `ROUND_UP_UNIT_VND` becomes `ROUND_UP_UNIT`.

## Edge cases covered
Boundary 1 cent; amounts that are multiples of 100 (no round-up); percentage fee rounding at 99 and 10,000; split with remainder; mobile maximum; parse rejections; display of 0 and of missing values.

## Architecture impact
`amend`: `architecture/crosscutting-concepts.md` (money section) and `domain/*` pages named in the ADR. No new component. The ADR is `proposed` until the work ships.

## User guide impact
`updated`: every chapter that shows an amount (dashboard, offers, plans and limits, workflow, reconciliation, organisations, fraud, KYC). Screenshots are retaken on the converted demo data. The mobile guide does not exist yet.
