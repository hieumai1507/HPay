---
name: api
description: "Skill for the Api area of hpay. 88 symbols across 23 files."
---

# Api

88 symbols | 23 files | Cohesion: 88%

## When to Use

- Working with code in `hpay-backend/`
- Understanding how RequireAdmin, NewAdminHandler, NewApprovalHandler work
- Modifying api-related functionality

## Key Files

| File | Symbols |
|------|---------|
| `hpay-backend/internal/api/request_handler.go` | Create, CreateSplit, Inbox, Sent, Detail (+5) |
| `hpay-backend/internal/api/admin_handler.go` | mustAdminID, Login, KYCQueue, KYCReview, ServeFile (+2) |
| `hpay-backend/internal/api/admin_ops_handler.go` | FraudFlags, ResolveFraudFlag, CreateReversal, AdminApprovals, AdminDecide (+2) |
| `hpay-backend/internal/api/org_handler.go` | Create, AddMember, CreatePolicy, Topup, CreateDisbursement (+2) |
| `hpay-mobile/src/lib/errors.ts` | ApiError, getErrorMessage, constructor, isApiError, isNetworkError (+2) |
| `hpay-backend/internal/api/auth_handler.go` | Register, Login, Refresh, Logout, deviceID (+1) |
| `hpay-backend/internal/api/wallet_handler.go` | List, Topup, History, Detail, mustUserID (+1) |
| `hpay-backend/internal/api/approval_handler.go` | Inbox, Challenge, Decide, approvalIDParam, NewApprovalHandler |
| `hpay-backend/internal/api/router.go` | NewRouter, requestID, accessLog, healthz, statusWord |
| `hpay-backend/internal/api/kyc_handler.go` | Submit, Status, readImagePart, NewKYCHandler |

## Entry Points

Start here when exploring this area:

- **`RequireAdmin`** (Function) — `hpay-backend/internal/api/admin_handler.go:20`
- **`NewAdminHandler`** (Function) — `hpay-backend/internal/api/admin_handler.go:55`
- **`NewApprovalHandler`** (Function) — `hpay-backend/internal/api/approval_handler.go:16`
- **`NewAuthHandler`** (Function) — `hpay-backend/internal/api/auth_handler.go:21`
- **`NewKYCHandler`** (Function) — `hpay-backend/internal/api/kyc_handler.go:16`

## Key Symbols

| Symbol | Type | File | Line |
|--------|------|------|------|
| `ApiError` | Class | `hpay-mobile/src/lib/errors.ts` | 69 |
| `NetworkError` | Class | `hpay-mobile/src/lib/errors.ts` | 84 |
| `RequireAdmin` | Function | `hpay-backend/internal/api/admin_handler.go` | 20 |
| `NewAdminHandler` | Function | `hpay-backend/internal/api/admin_handler.go` | 55 |
| `NewApprovalHandler` | Function | `hpay-backend/internal/api/approval_handler.go` | 16 |
| `NewAuthHandler` | Function | `hpay-backend/internal/api/auth_handler.go` | 21 |
| `NewKYCHandler` | Function | `hpay-backend/internal/api/kyc_handler.go` | 16 |
| `RequireAuth` | Function | `hpay-backend/internal/api/middleware.go` | 15 |
| `RateLimit` | Function | `hpay-backend/internal/api/middleware.go` | 38 |
| `RequireIdempotencyKey` | Function | `hpay-backend/internal/api/middleware.go` | 57 |
| `NewOrgHandler` | Function | `hpay-backend/internal/api/org_handler.go` | 16 |
| `NewRequestHandler` | Function | `hpay-backend/internal/api/request_handler.go` | 15 |
| `NewRouter` | Function | `hpay-backend/internal/api/router.go` | 41 |
| `NewTransferHandler` | Function | `hpay-backend/internal/api/transfer_handler.go` | 17 |
| `NewWalletHandler` | Function | `hpay-backend/internal/api/wallet_handler.go` | 16 |
| `NewAdminService` | Function | `hpay-backend/internal/auth/admin.go` | 22 |
| `RegisterExecutor` | Function | `hpay-backend/internal/reversal/reversal.go` | 45 |
| `NewSigner` | Function | `hpay-backend/internal/storage/storage.go` | 101 |
| `TestSignerRoundTripExpiryAndTamper` | Function | `hpay-backend/internal/storage/storage_test.go` | 32 |
| `parseErrorBody` | Function | `hpay-mobile/src/api/client.ts` | 69 |

## Execution Flows

| Flow | Type | Steps |
|------|------|-------|
| `RootLayout → GetDeviceId` | cross_community | 6 |
| `RootLayout → GetAccessToken` | cross_community | 6 |
| `RootLayout → NetworkError` | cross_community | 6 |
| `RootLayout → GetStoredRefreshToken` | cross_community | 5 |
| `RootLayout → ApiError` | cross_community | 5 |
| `RootLayout → ParseErrorBody` | cross_community | 5 |
| `NewRouter → Queries` | cross_community | 4 |
| `NewRouter → JWTIssuer` | cross_community | 3 |
| `NewRouter → Service` | cross_community | 3 |
| `NewRouter → Engine` | cross_community | 3 |

## Connected Areas

| Area | Connections |
|------|-------------|
| Workflow | 16 calls |
| Auth | 2 calls |
| Stores | 1 calls |

## How to Explore

1. `context({name: "RequireAdmin"})` — see callers and callees
2. `query({search_query: "api"})` — find related execution flows
3. Read key files listed above for implementation details
4. `explain({target: "<file or symbol>"})` — persisted taint findings (source→sink data flows), when indexed with `--pdg`
