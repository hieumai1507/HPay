---
id: ADR-0019
title: The currency is US dollars, stored as integer cents
status: accepted
date: 2026-11-25
supersedes: []
superseded_by: []
spec: specs/w6-usd/spec.md
affects:
  - domain/ledger.md
  - domain/wallet.md
  - domain/transfer-and-fees.md
  - product/kyc-tiers-and-limits.md
  - reference/data-model.md
  - reference/glossary.md
  - reference/configuration.md
  - architecture/crosscutting-concepts.md
constitution: [C-01]
---

# ADR-0019: The currency is US dollars, stored as integer cents

Implemented 2026-11-26 (spec specs/w6-usd/spec.md, verification handoff/w6-usd-verify.md). One consequence found in review: the migrations were edited in place, so a database that already ran them is not converted and nothing guards against it (backlog R1).

An accepted ADR is immutable. This one amends a sentence of ADR-0003 (the currency and its unit) without changing its rule (integers in minor units); ADR-0003 keeps its status and gains a pointer to this record.

## Context
Everything in the repository assumes Vietnamese dong: wallets default to `'VND'` (migration 000002, `wallet/service.go`), the apps format `1,000,000 ₫`, seeded amounts, fees, limits and the round-up unit are dong, and ADR-0003 says "VND has no fractional unit". The product is presented in the style of US wallet apps (Venmo, Cash App) and the owner wants US dollars. The offer limits were in fact derived from an approved USD table multiplied by 25,000 (migration 000023, `product/kyc-tiers-and-limits.md`), so the original numbers are dollar amounts.

The database has not been run outside test and demo databases (owner statement, 2026-11-24), so existing migrations can be edited in place, as was done for the English names.

Dollars have a sub-unit, so three things that were trivial for dong now need an explicit rule: a percentage fee, an even split of a bill, and the round-up amount.

## Options considered
| Option | Pros | Cons | Cost / risk |
|--------|------|------|-------------|
| A. USD stored as integer cents (chosen) | Keeps ADR-0003's rule: integers end to end, exact `SUM(ledger)` reconciliation, no new column types. Display shows `$12.34`. | Every typed amount needs a decimal-aware parser; fee, split and round-up need explicit rounding rules; every number in seeds, docs and screenshots changes. | Medium: wide but mechanical, and fully covered by existing tests once converted. |
| B. Keep dong in the database, show dollars | Smallest diff. | Display and storage disagree; every amount would carry an exchange rate that is not real; the API would lie. | High: a rate that does not exist becomes a hidden business rule. |
| C. Multi-currency (a currency per wallet, a rate table) | Most general. | Needs FX rates, cross-currency postings and new limits semantics; far more than asked. | High: new bounded context. |
| D. Keep VND | No work. | Does not match the product the owner is presenting. | Zero cost, rejected by the owner. |

## Decision
Use option A.

- The only currency is `USD`. The unit is the cent: amount `1` is `$0.01`. Wallet currency defaults and the system wallets are `USD`. The default lives in one Go constant, `wallet.DefaultCurrency`; no package writes the literal.
- Existing numbers are converted at one cent for 250 dong (the 25,000 VND/USD of migration 000023), so limits return to the approved dollar table. P2P fixed fee is set to 25 cents (`$0.25`) by decision, not by conversion. Round-up unit default is 100 cents (`$1.00`), env `ROUND_UP_UNIT`.
- Rounding rules, all in integer math:
  - Percentage fee: `amount * basis_points / 10,000`, rounded **down** to a whole cent (unchanged formula, now on cents), then clamped by min and max.
  - Split bill: largest-remainder in cents (unchanged algorithm); every share must be at least 1 cent.
  - Round-up: next multiple of the unit, in cents.
- API and database: integers in cents. OpenAPI descriptions say "amount in cents".
- Clients: amounts are `bigint` or integers, formatted `$1,234.56` (negative `-$1,234.56`, missing value an em dash). Typed amounts accept comma grouping and at most two decimals (`12`, `12.5`, `12.50`, `1,234.56`); `1.234.567`, three decimals, signs and letters are rejected. Parsing never uses `Number()` or `parseInt()` on user text; conversion to cents is by digit string.
- Largest values stay far under 2^53: the largest configured cap is `$100,000` a month, which is 10,000,000 cents.

## Constitution check
C-01 (integer minor units end to end): complied with and strengthened; the apps' upper bounds are restated in cents. C-12 (time zone) is unaffected. C-08 (migrations): edited in place because no real database exists; each edited migration keeps its down file. No deviation.

## Consequences
- Easier: the product reads as US dollars everywhere; limits match their approved source table.
- Harder: amount input has a decimal part, so tests for the parser and for rounding at 1 cent are needed (boundary cases listed in the spec).
- Edited migrations 000002, 000004 and 000023 are not safe for a database that already ran them. If one ever exists, a new migration must convert it (divide by 250 and set the currency); this is recorded in the runbook note of the spec, not built.
- Seeds, screenshots, guide, site and README change in the same release.
- Not covered: more than one currency, exchange rates, sub-cent fees.

## Documentation to update
`domain/ledger.md`, `domain/wallet.md`, `domain/transfer-and-fees.md`, `product/kyc-tiers-and-limits.md`, `reference/data-model.md`, `reference/glossary.md`, `reference/configuration.md` (`ROUND_UP_UNIT`), `architecture/crosscutting-concepts.md`, the pointer in ADR-0003, `docs/adr/README.md`, the admin user guide and its screenshots, the site and the README.
