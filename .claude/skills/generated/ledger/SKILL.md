---
name: ledger
description: "Skill for the Ledger area of hpay. 7 symbols across 3 files."
---

# Ledger

7 symbols | 3 files | Cohesion: 93%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how ValidateLegs, TestValidateLegs, TestEdge_E14_SelfTransferRejected work
- Modifying ledger-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/ledger/ledger_test.go` | leg, TestValidateLegs, TestEdge_E14_SelfTransferRejected, errorsIs, Unwrap |
| `hpay-backend/internal/ledger/ledger.go` | ValidateLegs |
| `hpay-backend/internal/workflow/types.go` | Unwrap |

## Entry Points

Start here when exploring this area:

- **`ValidateLegs`** (Function) — `hpay-backend/internal/ledger/ledger.go:34`
- **`TestValidateLegs`** (Function) — `hpay-backend/internal/ledger/ledger_test.go:14`
- **`TestEdge_E14_SelfTransferRejected`** (Function) — `hpay-backend/internal/ledger/ledger_test.go:40`
- **`Unwrap`** (Method) — `hpay-backend/internal/ledger/ledger_test.go:53`
- **`Unwrap`** (Method) — `hpay-backend/internal/workflow/types.go:110`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `ValidateLegs` | Function | `hpay-backend/internal/ledger/ledger.go` | 34 |
| `TestValidateLegs` | Function | `hpay-backend/internal/ledger/ledger_test.go` | 14 |
| `TestEdge_E14_SelfTransferRejected` | Function | `hpay-backend/internal/ledger/ledger_test.go` | 40 |
| `Unwrap` | Method | `hpay-backend/internal/ledger/ledger_test.go` | 53 |
| `Unwrap` | Method | `hpay-backend/internal/workflow/types.go` | 110 |
| `leg` | Function | `hpay-backend/internal/ledger/ledger_test.go` | 10 |
| `errorsIs` | Function | `hpay-backend/internal/ledger/ledger_test.go` | 48 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `Execute → ValidateLegs` | cross_community | 4 |
| `Topup → ValidateLegs` | cross_community | 4 |
| `Topup → ValidateLegs` | cross_community | 4 |

## How to Explore

1. `context({name: "ValidateLegs"})` — see callers and callees
2. `query({search_query: "ledger"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
