---
name: approval
description: "Skill for the Approval area of hpay. 26 symbols across 11 files."
---

# Approval

26 symbols | 11 files | Cohesion: 79%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how Evaluate, TestEvaluate, AdminDecide work
- Modifying approval-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/approval/service.go` | Decide, otpPurpose, Challenge, eligibility, finalize (+4) |
| `hpay-backend/internal/store/gen/approval.sql.go` | GetApprovalRequestForUpdate, InsertApprovalAction, ListApprovalActions, SetApprovalCurrentLevel, HasApprovalAction (+1) |
| `hpay-backend/internal/approval/evaluate.go` | Evaluate, sortedTiers |
| `hpay-backend/internal/approval/evaluate_test.go` | snap, TestEvaluate |
| `hpay-backend/internal/approval/admin.go` | AdminDecide |
| `hpay-backend/internal/fraud/admin.go` | Resolve |
| `hpay-backend/internal/store/gen/auth.sql.go` | InsertAudit |
| `hpay-backend/internal/store/gen/fraud.sql.go` | ResolveFraudFlagGuarded |
| `hpay-backend/internal/store/gen/org.sql.go` | GetOrgMember |
| `hpay-backend/internal/store/gen/transactions.sql.go` | ResumeTransactionGuarded |

## Entry Points

Start here when exploring this area:

- **`Evaluate`** (Function) — `hpay-backend/internal/approval/evaluate.go:53`
- **`TestEvaluate`** (Function) — `hpay-backend/internal/approval/evaluate_test.go:6`
- **`AdminDecide`** (Method) — `hpay-backend/internal/approval/admin.go:31`
- **`Decide`** (Method) — `hpay-backend/internal/approval/service.go:218`
- **`Resolve`** (Method) — `hpay-backend/internal/fraud/admin.go:45`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `Evaluate` | Function | `hpay-backend/internal/approval/evaluate.go` | 53 |
| `TestEvaluate` | Function | `hpay-backend/internal/approval/evaluate_test.go` | 6 |
| `AdminDecide` | Method | `hpay-backend/internal/approval/admin.go` | 31 |
| `Decide` | Method | `hpay-backend/internal/approval/service.go` | 218 |
| `Resolve` | Method | `hpay-backend/internal/fraud/admin.go` | 45 |
| `GetApprovalRequestForUpdate` | Method | `hpay-backend/internal/store/gen/approval.sql.go` | 178 |
| `InsertApprovalAction` | Method | `hpay-backend/internal/store/gen/approval.sql.go` | 244 |
| `ListApprovalActions` | Method | `hpay-backend/internal/store/gen/approval.sql.go` | 346 |
| `SetApprovalCurrentLevel` | Method | `hpay-backend/internal/store/gen/approval.sql.go` | 539 |
| `InsertAudit` | Method | `hpay-backend/internal/store/gen/auth.sql.go` | 192 |
| `ResolveFraudFlagGuarded` | Method | `hpay-backend/internal/store/gen/fraud.sql.go` | 192 |
| `Challenge` | Method | `hpay-backend/internal/approval/service.go` | 151 |
| `HasApprovalAction` | Method | `hpay-backend/internal/store/gen/approval.sql.go` | 224 |
| `GetOrgMember` | Method | `hpay-backend/internal/store/gen/org.sql.go` | 30 |
| `ExpireDue` | Method | `hpay-backend/internal/approval/service.go` | 398 |
| `FinalizeApprovalRequest` | Method | `hpay-backend/internal/store/gen/approval.sql.go` | 64 |
| `ResumeTransactionGuarded` | Method | `hpay-backend/internal/store/gen/transactions.sql.go` | 332 |
| `GetInstance` | Method | `hpay-backend/internal/store/gen/workflow.sql.go` | 173 |
| `sortedTiers` | Function | `hpay-backend/internal/approval/evaluate.go` | 95 |
| `snap` | Function | `hpay-backend/internal/approval/evaluate_test.go` | 4 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `ExpireDue → FinalizeApprovalRequest` | intra_community | 4 |
| `ExpireDue → FinalizeApprovalRequestParams` | intra_community | 4 |
| `ExpireDue → Finalized` | intra_community | 4 |
| `Challenge → GetOrgMember` | intra_community | 3 |
| `Challenge → HasApprovalAction` | intra_community | 3 |
| `Challenge → GetOrgMemberParams` | intra_community | 3 |
| `Challenge → HasApprovalActionParams` | intra_community | 3 |
| `ExpireDue → InsertAudit` | cross_community | 3 |
| `ExpireDue → InsertAuditParams` | intra_community | 3 |

## Connected Areas

| Area | Connections |
|------|-------------|
| Workflow | 2 calls |

## How to Explore

1. `context({name: "Evaluate"})` — see callers and callees
2. `query({search_query: "approval"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
