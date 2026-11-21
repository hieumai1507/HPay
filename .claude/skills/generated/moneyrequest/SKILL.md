---
name: moneyrequest
description: "Skill for the Moneyrequest area of hpay. 16 symbols across 5 files."
---

# Moneyrequest

16 symbols | 5 files | Cohesion: 91%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how ValidateCustomSplit, TestValidateCustomSplit, SplitEven work
- Modifying moneyrequest-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/moneyrequest/service.go` | Create, CreateSplit, noteText, ts, Decline (+2) |
| `hpay-backend/internal/moneyrequest/split_test.go` | TestValidateCustomSplit, sum, TestSplitEven, TestSplitEvenRejectsTooManyShares |
| `hpay-backend/internal/moneyrequest/split.go` | ValidateCustomSplit, SplitEven |
| `hpay-backend/internal/moneyrequest/sweeper.go` | Run, ExpireDue |
| `hpay-backend/internal/store/gen/money_request.sql.go` | InsertMoneyRequest |

## Entry Points

Start here when exploring this area:

- **`ValidateCustomSplit`** (Function) — `hpay-backend/internal/moneyrequest/split.go:38`
- **`TestValidateCustomSplit`** (Function) — `hpay-backend/internal/moneyrequest/split_test.go:51`
- **`SplitEven`** (Function) — `hpay-backend/internal/moneyrequest/split.go:15`
- **`TestSplitEven`** (Function) — `hpay-backend/internal/moneyrequest/split_test.go:12`
- **`TestSplitEvenRejectsTooManyShares`** (Function) — `hpay-backend/internal/moneyrequest/split_test.go:42`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `ValidateCustomSplit` | Function | `hpay-backend/internal/moneyrequest/split.go` | 38 |
| `TestValidateCustomSplit` | Function | `hpay-backend/internal/moneyrequest/split_test.go` | 51 |
| `SplitEven` | Function | `hpay-backend/internal/moneyrequest/split.go` | 15 |
| `TestSplitEven` | Function | `hpay-backend/internal/moneyrequest/split_test.go` | 12 |
| `TestSplitEvenRejectsTooManyShares` | Function | `hpay-backend/internal/moneyrequest/split_test.go` | 42 |
| `Create` | Method | `hpay-backend/internal/moneyrequest/service.go` | 51 |
| `CreateSplit` | Method | `hpay-backend/internal/moneyrequest/service.go` | 89 |
| `InsertMoneyRequest` | Method | `hpay-backend/internal/store/gen/money_request.sql.go` | 185 |
| `Decline` | Method | `hpay-backend/internal/moneyrequest/service.go` | 263 |
| `Cancel` | Method | `hpay-backend/internal/moneyrequest/service.go` | 275 |
| `Run` | Method | `hpay-backend/internal/moneyrequest/sweeper.go` | 22 |
| `ExpireDue` | Method | `hpay-backend/internal/moneyrequest/sweeper.go` | 43 |
| `noteText` | Function | `hpay-backend/internal/moneyrequest/service.go` | 347 |
| `ts` | Function | `hpay-backend/internal/moneyrequest/service.go` | 354 |
| `sum` | Function | `hpay-backend/internal/moneyrequest/split_test.go` | 4 |
| `declineOrCancelError` | Method | `hpay-backend/internal/moneyrequest/service.go` | 288 |

## How to Explore

1. `context({name: "ValidateCustomSplit"})` — see callers and callees
2. `query({search_query: "moneyrequest"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
