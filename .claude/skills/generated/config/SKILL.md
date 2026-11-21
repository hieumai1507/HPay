---
name: config
description: "Skill for the Config area of hpay. 4 symbols across 1 files."
---

# Config

4 symbols | 1 files | Cohesion: 86%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how Load, Valid work
- Modifying config-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/platform/config/config.go` | Valid, Load, getenv, getenvInt |

## Entry Points

Start here when exploring this area:

- **`Load`** (Function) — `hpay-backend/internal/platform/config/config.go:54`
- **`Valid`** (Method) — `hpay-backend/internal/platform/config/config.go:22`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `Load` | Function | `hpay-backend/internal/platform/config/config.go` | 54 |
| `Valid` | Method | `hpay-backend/internal/platform/config/config.go` | 22 |
| `getenv` | Function | `hpay-backend/internal/platform/config/config.go` | 88 |
| `getenvInt` | Function | `hpay-backend/internal/platform/config/config.go` | 95 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `Main → Config` | cross_community | 4 |
| `Main → Getenv` | cross_community | 4 |
| `Main → GetenvInt` | cross_community | 4 |
| `Main → Valid` | cross_community | 4 |

## How to Explore

1. `context({name: "Load"})` — see callers and callees
2. `query({search_query: "config"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
