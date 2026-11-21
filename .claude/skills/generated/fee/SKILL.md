---
name: fee
description: "Skill for the Fee area of hpay. 3 symbols across 2 files."
---

# Fee

3 symbols | 2 files | Cohesion: 100%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how Calc, TestCalc, For work
- Modifying fee-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/fee/fee.go` | Calc, For |
| `hpay-backend/internal/fee/fee_test.go` | TestCalc |

## Entry Points

Start here when exploring this area:

- **`Calc`** (Function) — `hpay-backend/internal/fee/fee.go:24`
- **`TestCalc`** (Function) — `hpay-backend/internal/fee/fee_test.go:4`
- **`For`** (Method) — `hpay-backend/internal/fee/fee.go:44`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `Calc` | Function | `hpay-backend/internal/fee/fee.go` | 24 |
| `TestCalc` | Function | `hpay-backend/internal/fee/fee_test.go` | 4 |
| `For` | Method | `hpay-backend/internal/fee/fee.go` | 44 |

## How to Explore

1. `context({name: "Calc"})` — see callers and callees
2. `query({search_query: "fee"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
