---
title: ADR 0003 Money as int64 minor units
type: decision
status: verified-against-code
last_verified: 2026-11-21
---

# ADR 0003: Money as int64 minor units

Status: Accepted (verified in code 2026-11-21)

Note (2026-11-25): the currency named below (VND, no fractional unit) is replaced by US dollars stored as cents in [ADR-0019](0019-usd-currency-in-cents.md) once that ADR is accepted. The rule of this record, integers in minor units, is unchanged.

## Context
A ledger-based wallet must never lose or invent value through rounding. Floats and decimal strings invite precision and overflow bugs. The currency is VND, which has no fractional unit in practice.

## Decision
All money is an integer count of minor units (for VND, 1 = 1 dong):
- Database: `BIGINT` columns with checks (`transactions.amount BIGINT CHECK (amount > 0)`, `fee BIGINT ... CHECK (fee >= 0)`, `ledger_entries.amount BIGINT CHECK (amount > 0)`, `balance_after BIGINT`, `wallets.balance BIGINT`, offer limits such as `primary_pocket_max BIGINT`). No `NUMERIC`, no float.
- Go: `int64` (for example `ledger.Leg.Amount int64`, `wallet.TxnLimit` returns `int64`).
- API: JSON numbers in minor units.
- Mobile: BigInt helpers (`money.ts`, `TestEdge_M06_*` tests); admin should follow.
- Rounding rules (fee rounds down) must be explicit in the fee module.

## Consequences
- Exact arithmetic and simple `SUM(ledger)` reconciliation (the seeder and recon assert it).
- int64 overflow is not guarded today (audit: ledger `Post` and `fee.Calc` Medium); JSON numbers above 2^53 lose precision in JavaScript, so clients must parse amounts as BigInt or strings when large.
- Fee and split division need an explicit remainder rule (open: audit section 5, Fee).
- Audit found `Number()`/`parseInt()` on amounts in several mobile screens, violating this decision (open).