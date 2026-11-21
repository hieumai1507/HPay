---
name: cluster-81
description: "Skill for the Cluster_81 area of hpay. 3 symbols across 1 files."
---

# Cluster_81

3 symbols | 1 files | Cohesion: 100%

## When to Use

- Working with code in `hpay-mobile/`
- Understanding how pollUntilSettled, watch work
- Modifying cluster_81-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-mobile/src/lib/polling.ts` | sleep, pollUntilSettled, watch |

## Entry Points

Start here when exploring this area:

- **`pollUntilSettled`** (Function) — `hpay-mobile/src/lib/polling.ts:34`
- **`watch`** (Method) — `hpay-mobile/src/lib/polling.ts:77`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `pollUntilSettled` | Function | `hpay-mobile/src/lib/polling.ts` | 34 |
| `watch` | Method | `hpay-mobile/src/lib/polling.ts` | 77 |
| `sleep` | Function | `hpay-mobile/src/lib/polling.ts` | 16 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `Watch → Sleep` | intra_community | 3 |

## How to Explore

1. `context({name: "pollUntilSettled"})` — see callers and callees
2. `query({search_query: "cluster_81"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
