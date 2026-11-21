---
name: stores
description: "Skill for the Stores area of hpay. 7 symbols across 5 files."
---

# Stores

7 symbols | 5 files | Cohesion: 86%

## When to Use

- Working with code in `hpay-mobile/`
- Understanding how refreshAccessToken, useSessionBootstrap, ensureDeviceId work
- Modifying stores-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-mobile/src/api/refresh.ts` | refreshAccessToken, useSessionBootstrap |
| `hpay-mobile/src/stores/session.ts` | useSessionStore, getStoredRefreshToken |
| `hpay-mobile/src/stores/device.ts` | ensureDeviceId |
| `hpay-mobile/app/_layout.tsx` | RootLayout |
| `hpay-mobile/app/index.tsx` | Index |

## Entry Points

Start here when exploring this area:

- **`refreshAccessToken`** (Function) — `hpay-mobile/src/api/refresh.ts:59`
- **`useSessionBootstrap`** (Function) — `hpay-mobile/src/api/refresh.ts:77`
- **`ensureDeviceId`** (Function) — `hpay-mobile/src/stores/device.ts:22`
- **`useSessionStore`** (Function) — `hpay-mobile/src/stores/session.ts:22`
- **`getStoredRefreshToken`** (Function) — `hpay-mobile/src/stores/session.ts:43`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `refreshAccessToken` | Function | `hpay-mobile/src/api/refresh.ts` | 59 |
| `useSessionBootstrap` | Function | `hpay-mobile/src/api/refresh.ts` | 77 |
| `ensureDeviceId` | Function | `hpay-mobile/src/stores/device.ts` | 22 |
| `useSessionStore` | Function | `hpay-mobile/src/stores/session.ts` | 22 |
| `getStoredRefreshToken` | Function | `hpay-mobile/src/stores/session.ts` | 43 |
| `RootLayout` | Function | `hpay-mobile/app/_layout.tsx` | 13 |
| `Index` | Function | `hpay-mobile/app/index.tsx` | 8 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `RootLayout → GetDeviceId` | cross_community | 6 |
| `RootLayout → GetAccessToken` | cross_community | 6 |
| `RootLayout → NetworkError` | cross_community | 6 |
| `RootLayout → GetStoredRefreshToken` | cross_community | 5 |
| `RootLayout → ApiError` | cross_community | 5 |
| `RootLayout → ParseErrorBody` | cross_community | 5 |
| `RootLayout → UseSessionStore` | intra_community | 3 |
| `RootLayout → EnsureDeviceId` | intra_community | 3 |

## Connected Areas

| Area | Connections |
|------|-------------|
| Api | 1 calls |

## How to Explore

1. `context({name: "refreshAccessToken"})` — see callers and callees
2. `query({search_query: "stores"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
