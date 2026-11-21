---
name: storage
description: "Skill for the Storage area of hpay. 6 symbols across 1 files."
---

# Storage

6 symbols | 1 files | Cohesion: 100%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how Put, Get, SignURL work
- Modifying storage-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/storage/storage.go` | safePath, Put, Get, mac, SignURL (+1) |

## Entry Points

Start here when exploring this area:

- **`Put`** (Method) — `hpay-backend/internal/storage/storage.go:67`
- **`Get`** (Method) — `hpay-backend/internal/storage/storage.go:78`
- **`SignURL`** (Method) — `hpay-backend/internal/storage/storage.go:112`
- **`Verify`** (Method) — `hpay-backend/internal/storage/storage.go:119`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `Put` | Method | `hpay-backend/internal/storage/storage.go` | 67 |
| `Get` | Method | `hpay-backend/internal/storage/storage.go` | 78 |
| `SignURL` | Method | `hpay-backend/internal/storage/storage.go` | 112 |
| `Verify` | Method | `hpay-backend/internal/storage/storage.go` | 119 |
| `safePath` | Method | `hpay-backend/internal/storage/storage.go` | 48 |
| `mac` | Method | `hpay-backend/internal/storage/storage.go` | 103 |

## How to Explore

1. `context({name: "Put"})` — see callers and callees
2. `query({search_query: "storage"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
