---
name: org
description: "Skill for the Org area of hpay. 17 symbols across 6 files."
---

# Org

17 symbols | 6 files | Cohesion: 81%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how TxnLimit, NewRefCode, TestEdge_E13_LimitByKycTier work
- Modifying org-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/org/service.go` | AddMember, Topup, duplicateTopup, CreateDisbursement, requireRole (+2) |
| `hpay-backend/internal/wallet/service.go` | TxnLimit, Topup, NewRefCode |
| `hpay-backend/internal/store/gen/transactions.sql.go` | CompleteTransaction, InsertTransaction |
| `hpay-backend/internal/transfer/service.go` | Transfer, noteText |
| `hpay-backend/internal/wallet/service_test.go` | TestEdge_E13_LimitByKycTier, TestNewRefCode |
| `hpay-backend/internal/workflow/handlers/handlers.go` | Execute |

## Entry Points

Start here when exploring this area:

- **`TxnLimit`** (Function) — `hpay-backend/internal/wallet/service.go:31`
- **`NewRefCode`** (Function) — `hpay-backend/internal/wallet/service.go:184`
- **`TestEdge_E13_LimitByKycTier`** (Function) — `hpay-backend/internal/wallet/service_test.go:10`
- **`TestNewRefCode`** (Function) — `hpay-backend/internal/wallet/service_test.go:29`
- **`AddMember`** (Method) — `hpay-backend/internal/org/service.go:103`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `TxnLimit` | Function | `hpay-backend/internal/wallet/service.go` | 31 |
| `NewRefCode` | Function | `hpay-backend/internal/wallet/service.go` | 184 |
| `TestEdge_E13_LimitByKycTier` | Function | `hpay-backend/internal/wallet/service_test.go` | 10 |
| `TestNewRefCode` | Function | `hpay-backend/internal/wallet/service_test.go` | 29 |
| `AddMember` | Method | `hpay-backend/internal/org/service.go` | 103 |
| `Topup` | Method | `hpay-backend/internal/org/service.go` | 217 |
| `CreateDisbursement` | Method | `hpay-backend/internal/org/service.go` | 301 |
| `CompleteTransaction` | Method | `hpay-backend/internal/store/gen/transactions.sql.go` | 21 |
| `InsertTransaction` | Method | `hpay-backend/internal/store/gen/transactions.sql.go` | 209 |
| `Transfer` | Method | `hpay-backend/internal/transfer/service.go` | 57 |
| `Topup` | Method | `hpay-backend/internal/wallet/service.go` | 82 |
| `Execute` | Method | `hpay-backend/internal/workflow/handlers/handlers.go` | 148 |
| `noteText` | Function | `hpay-backend/internal/org/service.go` | 416 |
| `noteText` | Function | `hpay-backend/internal/transfer/service.go` | 185 |
| `duplicateTopup` | Method | `hpay-backend/internal/org/service.go` | 275 |
| `requireRole` | Method | `hpay-backend/internal/org/service.go` | 388 |
| `requireActiveOrg` | Method | `hpay-backend/internal/org/service.go` | 402 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `Topup → Queries` | cross_community | 4 |
| `Topup → GetWalletsForUpdate` | cross_community | 4 |
| `Topup → InsertLedgerEntry` | cross_community | 4 |
| `Topup → ApplyWalletDelta` | cross_community | 4 |
| `Topup → ValidateLegs` | cross_community | 4 |
| `Topup → Queries` | cross_community | 4 |
| `Topup → GetWalletsForUpdate` | cross_community | 4 |
| `Topup → InsertLedgerEntry` | cross_community | 4 |
| `Topup → ApplyWalletDelta` | cross_community | 4 |
| `Topup → ValidateLegs` | cross_community | 4 |

## Connected Areas

| Area | Connections |
|------|-------------|
| Gen | 2 calls |
| Workflow | 1 calls |

## How to Explore

1. `context({name: "TxnLimit"})` — see callers and callees
2. `query({search_query: "org"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
