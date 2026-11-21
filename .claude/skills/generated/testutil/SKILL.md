---
name: testutil
description: "Skill for the Testutil area of hpay. 3 symbols across 3 files."
---

# Testutil

3 symbols | 3 files | Cohesion: 100%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how RunSerialized, TestMain, TestMain work
- Modifying testutil-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/testutil/testutil.go` | RunSerialized |
| `hpay-backend/internal/workflow/main_test.go` | TestMain |
| `hpay-backend/test/integration/helpers_test.go` | TestMain |

## Entry Points

Start here when exploring this area:

- **`RunSerialized`** (Function) — `hpay-backend/internal/testutil/testutil.go:30`
- **`TestMain`** (Function) — `hpay-backend/internal/workflow/main_test.go:11`
- **`TestMain`** (Function) — `hpay-backend/test/integration/helpers_test.go:39`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `RunSerialized` | Function | `hpay-backend/internal/testutil/testutil.go` | 30 |
| `TestMain` | Function | `hpay-backend/internal/workflow/main_test.go` | 11 |
| `TestMain` | Function | `hpay-backend/test/integration/helpers_test.go` | 39 |

## How to Explore

1. `context({name: "RunSerialized"})` — see callers and callees
2. `query({search_query: "testutil"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
