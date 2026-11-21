---
name: outbox
description: "Skill for the Outbox area of hpay. 6 symbols across 3 files."
---

# Outbox

6 symbols | 3 files | Cohesion: 67%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how Run, RelayOnce, ClaimOutboxBatch work
- Modifying outbox-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/outbox/relay.go` | Run, RelayOnce |
| `hpay-backend/internal/store/gen/outbox.sql.go` | ClaimOutboxBatch, MarkOutboxPublished |
| `hpay-backend/internal/workflow/consumer.go` | processResultMessage, PumpLocal |

## Entry Points

Start here when exploring this area:

- **`Run`** (Method) — `hpay-backend/internal/outbox/relay.go:29`
- **`RelayOnce`** (Method) — `hpay-backend/internal/outbox/relay.go:46`
- **`ClaimOutboxBatch`** (Method) — `hpay-backend/internal/store/gen/outbox.sql.go:27`
- **`MarkOutboxPublished`** (Method) — `hpay-backend/internal/store/gen/outbox.sql.go:76`
- **`PumpLocal`** (Method) — `hpay-backend/internal/workflow/consumer.go:265`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `Run` | Method | `hpay-backend/internal/outbox/relay.go` | 29 |
| `RelayOnce` | Method | `hpay-backend/internal/outbox/relay.go` | 46 |
| `ClaimOutboxBatch` | Method | `hpay-backend/internal/store/gen/outbox.sql.go` | 27 |
| `MarkOutboxPublished` | Method | `hpay-backend/internal/store/gen/outbox.sql.go` | 76 |
| `PumpLocal` | Method | `hpay-backend/internal/workflow/consumer.go` | 265 |
| `processResultMessage` | Method | `hpay-backend/internal/workflow/consumer.go` | 236 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `Run → Queries` | cross_community | 4 |
| `Run → StepStream` | cross_community | 4 |
| `PumpLocal → GetStepExecutionParams` | cross_community | 3 |
| `PumpLocal → ParseSteps` | cross_community | 3 |
| `PumpLocal → StepIndex` | cross_community | 3 |
| `PumpLocal → StepInput` | cross_community | 3 |
| `Run → ClaimOutboxBatch` | intra_community | 3 |
| `Run → MarkOutboxPublished` | intra_community | 3 |

## Connected Areas

| Area | Connections |
|------|-------------|
| Workflow | 3 calls |

## How to Explore

1. `context({name: "Run"})` — see callers and callees
2. `query({search_query: "outbox"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
