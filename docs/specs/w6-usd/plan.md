---
title: Plan for US dollars stored as cents
type: plan
status: in-progress
last_verified: 2026-11-25
spec: spec.md
adr: ../../adr/0019-usd-currency-in-cents.md
---

# Plan: US dollars stored as cents

Approval: the owner chose "follow the recommendation" for the unit, the fee and the limits on 2026-11-25 and has a standing instruction to continue without asking. ADR-0019 stays `proposed` until stage 9.

## Tasks
Three code tasks have disjoint file sets and run in parallel in separate worktrees. Docs, guide and site depend on all three.

| Id | Task | Files owned | Depends on | Size | Proof | Effort |
|----|------|-------------|------------|------|-------|--------|
| T1 | Backend: constant `wallet.DefaultCurrency`; migrations 000002, 000004, 000023 in place; seed; round-up env `ROUND_UP_UNIT` (default 100); error texts; OpenAPI text; convert every backend test amount | `src/backend/**` | none | L | AC-1, 5, 6, 7, 8, 9 tests; full `-race` suite | high (money) |
| T2 | Admin: `lib/money.ts` to dollars (`formatUsd`, decimal-aware parser), six pages, unit tests, e2e text | `src/admin/**` | none | M | AC-2, 3, 4 unit tests; lint, tsc, build, e2e | medium |
| T3 | Mobile: `src/lib/money.ts` to dollars, `AmountInput`, `AmountDisplay`, screens, `MAX_AMOUNT` 4,000,000 cents, all tests | `src/mobile/**` | none | L | AC-2, 3, 4 jest tests; tsc, lint, jest | medium |
| T4 | Knowledge base: domain, reference, product, architecture pages, ADR pointers, glossary, configuration | `docs/**` except guide and specs | T1 | M | grep finds no stray VND outside `history/` and ADR text | low |
| T5 | Demo data, retake guide screenshots, rebuild guide HTML, run checker | `docs/guides/**` | T1, T2 | M | `check-guide.mjs` output | medium |
| T6 | Site and README: retake images, text | `site/**`, `README.md` | T5 | S | `site/check.mjs` | low |
| T7 | Verify (full suites, load test once), review, ADR to accepted | docs, specs | T1-T6 | M | verify note, performance note | medium |

## Order of work inside T1 (backend)
1. Red: convert the test amounts and add the new boundary tests (AC-3 is client side; backend: fee 99 and 10,000 at 25 bp, split 10.00 by three, round-up 12.34 and 12.00, fee fixed 25, currency constant). Run and paste the failures.
2. Green: constant, migrations, seed, handlers.
3. Fresh database: all migrations up, seed, ledger sum check.

## Constitution check
| Principle | Answer |
|-----------|--------|
| C-01 integer minor units | Complied with: cents are integers; parsers use digit strings; no `Number()` on text; bounds restated |
| C-02 to C-07 | Unchanged: no change to locking, idempotency, outbox, auth |
| C-08 migrations | Edited in place (no real database); each keeps its down file; verified by running up and down on a fresh database |
| C-12 time zone | Unchanged |
| C-13 | Unchanged |

## Rollout and rollback
Nothing is deployed. Rollback is `git revert`. If a database ever ran the old migrations, convert it with a new migration (divide amounts by 250, set the currency); not built because none exists.

## Risks
- Percentage fee and split rounding at 1 cent: covered by boundary tests.
- Missed literals: a final repository-wide search for `VND`, `₫`, `vnd`, `dong`.
- Admin `MAX_MINOR_UNITS` and mobile `MAX_AMOUNT` must stay below 2^53: 4,000,000 and 1e12 are.
- Tests that assert old formatted strings (`1,000,000 ₫`) fail until converted: part of T2 and T3.

## Performance gate
The change touches amounts on the money path but not its locking or cost. The load test is run once after T1 to confirm ledger integrity with the new numbers (stage 7).
