---
name: recon
description: "Skill for the Recon area of hpay. 9 symbols across 3 files."
---

# Recon

9 symbols | 3 files | Cohesion: 100%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how RunForDate, InsertReconItem, InsertReconRun work
- Modifying recon-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/recon/recon.go` | RunForDate, classify, countResults, summarize, sameDate |
| `hpay-backend/internal/store/gen/recon.sql.go` | InsertReconItem, InsertReconRun |
| `hpay-backend/internal/recon/xlsx.go` | buildReport, itoa64 |

## Entry Points

Start here when exploring this area:

- **`RunForDate`** (Method) — `hpay-backend/internal/recon/recon.go:53`
- **`InsertReconItem`** (Method) — `hpay-backend/internal/store/gen/recon.sql.go:67`
- **`InsertReconRun`** (Method) — `hpay-backend/internal/store/gen/recon.sql.go:99`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `RunForDate` | Method | `hpay-backend/internal/recon/recon.go` | 53 |
| `InsertReconItem` | Method | `hpay-backend/internal/store/gen/recon.sql.go` | 67 |
| `InsertReconRun` | Method | `hpay-backend/internal/store/gen/recon.sql.go` | 99 |
| `classify` | Function | `hpay-backend/internal/recon/recon.go` | 134 |
| `countResults` | Function | `hpay-backend/internal/recon/recon.go` | 195 |
| `sameDate` | Function | `hpay-backend/internal/recon/recon.go` | 252 |
| `itoa64` | Function | `hpay-backend/internal/recon/xlsx.go` | 53 |
| `summarize` | Method | `hpay-backend/internal/recon/recon.go` | 206 |
| `buildReport` | Method | `hpay-backend/internal/recon/xlsx.go` | 13 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `RunForDate → ClassifiedItem` | intra_community | 3 |
| `RunForDate → SameDate` | intra_community | 3 |

## How to Explore

1. `context({name: "RunForDate"})` — see callers and callees
2. `query({search_query: "recon"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
