---
name: kyc
description: "Skill for the Kyc area of hpay. 12 symbols across 6 files."
---

# Kyc

12 symbols | 6 files | Cohesion: 92%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how Classify, TestClassifyThresholds, TestParseScoreJSON work
- Modifying kyc-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/kyc/scorer_claude.go` | Score, imageBlock, parseScoreJSON |
| `hpay-backend/internal/kyc/handlers.go` | requestIDFromContext, Execute |
| `hpay-backend/internal/kyc/kyc_test.go` | TestClassifyThresholds, TestParseScoreJSON |
| `hpay-backend/internal/kyc/service.go` | ValidateImage, Submit |
| `hpay-backend/internal/store/gen/kyc.sql.go` | InsertKYCRequest, MarkUserKycPending |
| `hpay-backend/internal/kyc/classify.go` | Classify |

## Entry Points

Start here when exploring this area:

- **`Classify`** (Function) — `hpay-backend/internal/kyc/classify.go:21`
- **`TestClassifyThresholds`** (Function) — `hpay-backend/internal/kyc/kyc_test.go:12`
- **`TestParseScoreJSON`** (Function) — `hpay-backend/internal/kyc/kyc_test.go:50`
- **`ValidateImage`** (Function) — `hpay-backend/internal/kyc/service.go:40`
- **`Execute`** (Method) — `hpay-backend/internal/kyc/handlers.go:44`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `Classify` | Function | `hpay-backend/internal/kyc/classify.go` | 21 |
| `TestClassifyThresholds` | Function | `hpay-backend/internal/kyc/kyc_test.go` | 12 |
| `TestParseScoreJSON` | Function | `hpay-backend/internal/kyc/kyc_test.go` | 50 |
| `ValidateImage` | Function | `hpay-backend/internal/kyc/service.go` | 40 |
| `Execute` | Method | `hpay-backend/internal/kyc/handlers.go` | 44 |
| `Score` | Method | `hpay-backend/internal/kyc/scorer_claude.go` | 52 |
| `Submit` | Method | `hpay-backend/internal/kyc/service.go` | 64 |
| `InsertKYCRequest` | Method | `hpay-backend/internal/store/gen/kyc.sql.go` | 123 |
| `MarkUserKycPending` | Method | `hpay-backend/internal/store/gen/kyc.sql.go` | 194 |
| `requestIDFromContext` | Function | `hpay-backend/internal/kyc/handlers.go` | 25 |
| `imageBlock` | Function | `hpay-backend/internal/kyc/scorer_claude.go` | 111 |
| `parseScoreJSON` | Function | `hpay-backend/internal/kyc/scorer_claude.go` | 124 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `Score → Result` | intra_community | 3 |

## How to Explore

1. `context({name: "Classify"})` — see callers and callees
2. `query({search_query: "kyc"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
