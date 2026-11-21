---
name: integration
description: "Skill for the Integration area of hpay. 48 symbols across 11 files."
---

# Integration

48 symbols | 11 files | Cohesion: 65%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how WalletBalance, TestOrgDisbursementTwoTierEndToEnd, TestOrgDisbursementRejectFailsClean work
- Modifying integration-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/test/integration/approval_test.go` | newOrgFixture, approvalRequestFor, txnStatus, approvalStatus, decide (+6) |
| `hpay-backend/test/integration/recon_test.go` | uniqueReconDay, mkTopup, mkStatement, seedSystemWalletID, TestEdge_E16_ReconCutoffNoFalseMismatch (+4) |
| `hpay-backend/test/integration/kyc_test.go` | TestKYCTierUnlocksTransferLimit, submitAndDrain, TestKYCAutoApproveBranch, TestKYCManualThenAdminApprove, TestKYCRejectBranchAllowsResubmit (+2) |
| `hpay-backend/test/integration/helpers_test.go` | postTransfer, fund, setScore, drain, errIs |
| `hpay-backend/test/integration/fraud_test.go` | verifyKYC, insertP2PHistory, TestFraudBlocksHighRisk, TestFraudFlagsGreyZone |
| `hpay-backend/internal/testutil/testutil.go` | WalletBalance, PoolFromEnv, LedgerSum |
| `hpay-backend/test/integration/moneyrequest_test.go` | TestMoneyRequestPayEndToEnd, TestMoneyRequestCancelAndActorGuards, TestEdge_E19_ExpiryRacePay |
| `hpay-backend/test/integration/reversal_test.go` | TestReversalEndToEnd, TestReversalGuards |
| `hpay-backend/test/integration/ledger_test.go` | TestEdge_E02_ConcurrentOverdraw, TestEdge_E03_CrossTransferNoDeadlock |
| `hpay-backend/test/integration/transfer_test.go` | TestEdge_E12_LockedUserFailsMidWorkflow |

## Entry Points

Start here when exploring this area:

- **`WalletBalance`** (Function) — `hpay-backend/internal/testutil/testutil.go:98`
- **`TestOrgDisbursementTwoTierEndToEnd`** (Function) — `hpay-backend/test/integration/approval_test.go:112`
- **`TestOrgDisbursementRejectFailsClean`** (Function) — `hpay-backend/test/integration/approval_test.go:188`
- **`TestEdge_E08_CheckerRaceSingleFinalize`** (Function) — `hpay-backend/test/integration/approval_test.go:223`
- **`TestEdge_E09_PolicySnapshotSurvivesPolicyChange`** (Function) — `hpay-backend/test/integration/approval_test.go:293`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `WalletBalance` | Function | `hpay-backend/internal/testutil/testutil.go` | 98 |
| `TestOrgDisbursementTwoTierEndToEnd` | Function | `hpay-backend/test/integration/approval_test.go` | 112 |
| `TestOrgDisbursementRejectFailsClean` | Function | `hpay-backend/test/integration/approval_test.go` | 188 |
| `TestEdge_E08_CheckerRaceSingleFinalize` | Function | `hpay-backend/test/integration/approval_test.go` | 223 |
| `TestEdge_E09_PolicySnapshotSurvivesPolicyChange` | Function | `hpay-backend/test/integration/approval_test.go` | 293 |
| `TestEdge_E10_ExpiredApprovalFailsClean` | Function | `hpay-backend/test/integration/approval_test.go` | 346 |
| `TestApprovalEligibility` | Function | `hpay-backend/test/integration/approval_test.go` | 410 |
| `TestMoneyRequestPayEndToEnd` | Function | `hpay-backend/test/integration/moneyrequest_test.go` | 16 |
| `TestEdge_E12_LockedUserFailsMidWorkflow` | Function | `hpay-backend/test/integration/transfer_test.go` | 122 |
| `TestEdge_E16_ReconCutoffNoFalseMismatch` | Function | `hpay-backend/test/integration/recon_test.go` | 59 |
| `TestReconFourCategoriesAndIdempotent` | Function | `hpay-backend/test/integration/recon_test.go` | 96 |
| `TestReversalEndToEnd` | Function | `hpay-backend/test/integration/reversal_test.go` | 16 |
| `PoolFromEnv` | Function | `hpay-backend/internal/testutil/testutil.go` | 53 |
| `LedgerSum` | Function | `hpay-backend/internal/testutil/testutil.go` | 108 |
| `TestKYCTierUnlocksTransferLimit` | Function | `hpay-backend/test/integration/kyc_test.go` | 181 |
| `TestEdge_E02_ConcurrentOverdraw` | Function | `hpay-backend/test/integration/ledger_test.go` | 14 |
| `TestEdge_E03_CrossTransferNoDeadlock` | Function | `hpay-backend/test/integration/ledger_test.go` | 57 |
| `TestReversalGuards` | Function | `hpay-backend/test/integration/reversal_test.go` | 94 |
| `TestEdge_E01_IdempotentTopup` | Function | `hpay-backend/test/integration/wallet_test.go` | 14 |
| `TestKYCAutoApproveBranch` | Function | `hpay-backend/test/integration/kyc_test.go` | 59 |

## Connected Areas

| Area | Connections |
|------|-------------|
| Workflow | 27 calls |
| Gen | 13 calls |
| Org | 4 calls |

## How to Explore

1. `context({name: "WalletBalance"})` — see callers and callees
2. `query({search_query: "integration"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
