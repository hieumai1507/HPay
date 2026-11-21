---
name: fraud
description: "Skill for the Fraud area of hpay. 12 symbols across 4 files."
---

# Fraud

12 symbols | 4 files | Cohesion: 91%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how Decide, Evaluate, TestEvaluateAndDecide work
- Modifying fraud-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/fraud/admin.go` | pgts, itoa, List, parseFlagStatus |
| `hpay-backend/internal/fraud/handler.go` | Execute, enqueueAdvisory, topRuleCode |
| `hpay-backend/internal/fraud/rules.go` | Decide, Evaluate, max64 |
| `hpay-backend/internal/fraud/rules_test.go` | TestEvaluateAndDecide, TestAmountSpikeNeedsHistory |

## Entry Points

Start here when exploring this area:

- **`Decide`** (Function) — `hpay-backend/internal/fraud/rules.go:89`
- **`Evaluate`** (Function) — `hpay-backend/internal/fraud/rules.go:34`
- **`TestEvaluateAndDecide`** (Function) — `hpay-backend/internal/fraud/rules_test.go:4`
- **`TestAmountSpikeNeedsHistory`** (Function) — `hpay-backend/internal/fraud/rules_test.go:45`
- **`Execute`** (Method) — `hpay-backend/internal/fraud/handler.go:35`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `Decide` | Function | `hpay-backend/internal/fraud/rules.go` | 89 |
| `Evaluate` | Function | `hpay-backend/internal/fraud/rules.go` | 34 |
| `TestEvaluateAndDecide` | Function | `hpay-backend/internal/fraud/rules_test.go` | 4 |
| `TestAmountSpikeNeedsHistory` | Function | `hpay-backend/internal/fraud/rules_test.go` | 45 |
| `Execute` | Method | `hpay-backend/internal/fraud/handler.go` | 35 |
| `List` | Method | `hpay-backend/internal/fraud/admin.go` | 34 |
| `pgts` | Function | `hpay-backend/internal/fraud/admin.go` | 98 |
| `itoa` | Function | `hpay-backend/internal/fraud/admin.go` | 100 |
| `topRuleCode` | Function | `hpay-backend/internal/fraud/handler.go` | 120 |
| `max64` | Function | `hpay-backend/internal/fraud/rules.go` | 100 |
| `parseFlagStatus` | Function | `hpay-backend/internal/fraud/admin.go` | 85 |
| `enqueueAdvisory` | Method | `hpay-backend/internal/fraud/handler.go` | 96 |

## How to Explore

1. `context({name: "Decide"})` — see callers and callees
2. `query({search_query: "fraud"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
