---
name: workflow
description: "Skill for the Workflow area of hpay. 102 symbols across 42 files."
---

# Workflow

102 symbols | 42 files | Cohesion: 65%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how NewService, NewService, NewService work
- Modifying workflow-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/workflow/engine.go` | NewEngine, SetPauseHook, Start, orEmpty, OnResult (+14) |
| `hpay-backend/internal/workflow/integration_test.go` | newHarness, insertDefinition, pumpOutbox, drain, instanceStatus (+8) |
| `hpay-backend/internal/workflow/consumer.go` | NewWorker, processStepMessage, emitRecordedResult, recordAndEmit, jsonbOrNil (+6) |
| `hpay-backend/internal/store/gen/workflow.sql.go` | GetDefinitionByID, ListStepExecutions, AdvanceInstance, FailTransactionGuarded, InsertStepExecution |
| `hpay-backend/internal/workflow/types.go` | Name, ParseSteps, StepIndex, IsRetryable |
| `hpay-backend/internal/workflow/recovery.go` | NewRecovery, Run, autoClaim, SweepStuck |
| `hpay-backend/internal/workflow/registry.go` | Register, NewRegistry, Names |
| `hpay-backend/internal/recon/cron.go` | NewCron, Run, nextRunAt |
| `hpay-backend/internal/transfer/service.go` | NewService, NewServiceWithDefinition |
| `hpay-backend/test/integration/transfer_test.go` | TestP2PTransferEndToEnd, TestEdge_E07_CompensationReversesMovedFunds |

## Entry Points

Start here when exploring this area:

- **`NewService`** (Function) — `hpay-backend/internal/approval/service.go:54`
- **`NewService`** (Function) — `hpay-backend/internal/auth/service.go:48`
- **`NewService`** (Function) — `hpay-backend/internal/fee/fee.go:40`
- **`NewAdminService`** (Function) — `hpay-backend/internal/fraud/admin.go:28`
- **`RegisterHandler`** (Function) — `hpay-backend/internal/fraud/handler.go:22`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `NewService` | Function | `hpay-backend/internal/approval/service.go` | 54 |
| `NewService` | Function | `hpay-backend/internal/auth/service.go` | 48 |
| `NewService` | Function | `hpay-backend/internal/fee/fee.go` | 40 |
| `NewAdminService` | Function | `hpay-backend/internal/fraud/admin.go` | 28 |
| `RegisterHandler` | Function | `hpay-backend/internal/fraud/handler.go` | 22 |
| `NewAdminService` | Function | `hpay-backend/internal/kyc/admin.go` | 30 |
| `RegisterHandlers` | Function | `hpay-backend/internal/kyc/handlers.go` | 18 |
| `NewService` | Function | `hpay-backend/internal/kyc/service.go` | 35 |
| `NewService` | Function | `hpay-backend/internal/moneyrequest/service.go` | 39 |
| `NewService` | Function | `hpay-backend/internal/org/service.go` | 50 |
| `NewService` | Function | `hpay-backend/internal/recon/recon.go` | 38 |
| `NewService` | Function | `hpay-backend/internal/reversal/reversal.go` | 39 |
| `New` | Function | `hpay-backend/internal/store/gen/db.go` | 19 |
| `SystemWallet` | Function | `hpay-backend/internal/testutil/testutil.go` | 88 |
| `NewService` | Function | `hpay-backend/internal/transfer/service.go` | 33 |
| `NewServiceWithDefinition` | Function | `hpay-backend/internal/transfer/service.go` | 38 |
| `NewService` | Function | `hpay-backend/internal/wallet/service.go` | 47 |
| `NewWorker` | Function | `hpay-backend/internal/workflow/consumer.go` | 39 |
| `NewEngine` | Function | `hpay-backend/internal/workflow/engine.go` | 37 |
| `RegisterAll` | Function | `hpay-backend/internal/workflow/handlers/handlers.go` | 20 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `NewRouter → Queries` | cross_community | 4 |
| `Execute → Queries` | cross_community | 4 |
| `Topup → Queries` | cross_community | 4 |
| `Topup → Queries` | cross_community | 4 |
| `Main → Config` | cross_community | 4 |
| `Main → Getenv` | cross_community | 4 |
| `Main → GetenvInt` | cross_community | 4 |
| `Main → Valid` | cross_community | 4 |
| `Run → Queries` | cross_community | 4 |
| `Run → StepStream` | cross_community | 4 |

## Connected Areas

| Area | Connections |
|------|-------------|
| Integration | 6 calls |
| Api | 4 calls |
| Outbox | 3 calls |
| Gen | 3 calls |
| Auth | 2 calls |
| Config | 1 calls |

## How to Explore

1. `context({name: "NewService"})` — see callers and callees
2. `query({search_query: "workflow"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
