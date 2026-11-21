---
name: gen
description: "Skill for the Gen area of hpay. 73 symbols across 24 files."
---

# Gen

73 symbols | 24 files | Cohesion: 75%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how Post, ValidateTiers, TestValidateTiers work
- Modifying gen-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/store/gen/models.go` | Scan, Scan, Scan, Scan, Scan (+12) |
| `hpay-backend/internal/store/gen/wallets.sql.go` | GetOwnerStatusByWalletID, GetWalletByID, GetWalletOwner, ApplyWalletDelta, GetWalletsForUpdate (+4) |
| `hpay-backend/internal/store/gen/approval.sql.go` | DeactivateOrgPoliciesByName, GetNextPolicyVersion, InsertApprovalPolicy, GetActivePolicyForAmount, InsertApprovalRequest |
| `hpay-backend/internal/store/gen/kyc.sql.go` | GetKYCRequest, ReviewKYCGuarded, SetUserKycFromManualReview, ClassifyKYCGuarded, SetUserKycFromPending |
| `hpay-backend/internal/store/gen/transactions.sql.go` | GetTransactionByWorkflow, PendTransactionGuarded, InsertReversalTransaction, MarkTransactionReversed |
| `hpay-backend/internal/workflow/handlers/handlers.go` | txnForStep, Execute, Execute, legs |
| `hpay-backend/internal/ledger/ledger.go` | Post, PostWithQueries, uniqueSortedIDs |
| `hpay-backend/internal/store/gen/workflow.sql.go` | GetInstanceForUpdate, GetStepExecution, ResumeRecoveredInstance |
| `hpay-backend/internal/workflow/engine.go` | ReEmitCurrentStep, maxRetry, nextFreeAttempt |
| `hpay-backend/internal/org/service.go` | CreatePolicy, CreateOrg |

## Entry Points

Start here when exploring this area:

- **`Post`** (Function) — `hpay-backend/internal/ledger/ledger.go:69`
- **`ValidateTiers`** (Function) — `hpay-backend/internal/approval/evaluate.go:75`
- **`TestValidateTiers`** (Function) — `hpay-backend/internal/approval/evaluate_test.go:47`
- **`PostWithQueries`** (Function) — `hpay-backend/internal/ledger/ledger.go:76`
- **`NewUserWithWallet`** (Function) — `hpay-backend/internal/testutil/testutil.go:68`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `Post` | Function | `hpay-backend/internal/ledger/ledger.go` | 69 |
| `ValidateTiers` | Function | `hpay-backend/internal/approval/evaluate.go` | 75 |
| `TestValidateTiers` | Function | `hpay-backend/internal/approval/evaluate_test.go` | 47 |
| `PostWithQueries` | Function | `hpay-backend/internal/ledger/ledger.go` | 76 |
| `NewUserWithWallet` | Function | `hpay-backend/internal/testutil/testutil.go` | 68 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 23 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 41 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 86 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 128 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 172 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 216 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 258 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 301 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 347 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 392 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 435 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 478 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 522 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 567 |
| `Scan` | Method | `hpay-backend/internal/store/gen/models.go` | 614 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `Execute → Queries` | cross_community | 4 |
| `Execute → GetWalletsForUpdate` | cross_community | 4 |
| `Execute → InsertLedgerEntry` | cross_community | 4 |
| `Execute → ApplyWalletDelta` | cross_community | 4 |
| `Execute → ValidateLegs` | cross_community | 4 |
| `Topup → Queries` | cross_community | 4 |
| `Topup → GetWalletsForUpdate` | cross_community | 4 |
| `Topup → InsertLedgerEntry` | cross_community | 4 |
| `Topup → ApplyWalletDelta` | cross_community | 4 |
| `Topup → ValidateLegs` | cross_community | 4 |

## Connected Areas

| Area | Connections |
|------|-------------|
| Workflow | 9 calls |
| Org | 5 calls |
| Approval | 4 calls |
| Kyc | 2 calls |
| Moneyrequest | 1 calls |
| Ledger | 1 calls |

## How to Explore

1. `context({name: "Post"})` — see callers and callees
2. `query({search_query: "gen"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
