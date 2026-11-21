---
title: HTTP API reference
type: reference
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/api/openapi.yaml, src/backend/internal/api/router.go, src/backend/internal/api/middleware.go, src/backend/scripts/check-openapi-sync.py]
---

# HTTP API reference

Source of truth: the mounted routes in `router.go`. The OpenAPI file declares the same 68 operations (see "Spec vs router mismatches").

## Surfaces and process roles

| Group | Mounted when role is | Auth |
|---|---|---|
| `/healthz` | any HTTP role | public |
| `/internal/mock-bank/callback`, `/v1/auth/*`, user routes (`/v1/users`, `/v1/wallets`, `/v1/transactions`, `/v1/fees`, `/v1/vaults`, `/v1/money-requests`, `/v1/kyc`, `/v1/qr`) | `api` or `all` | see tables |
| `/v1/org/*`, `/v1/admin/*`, `/v1/files/*` | `admin` or `all` | see tables |

Role `worker` serves no HTTP. Role comes from `--role=` on `cmd/server`.

## Conventions

- Auth: `RequireAuth` expects `Authorization: Bearer <access JWT>` (aud `user`). Missing header returns 401 `ERR_INVALID_TOKEN`. `RequireAdmin` expects an admin JWT (aud `admin`, same `JWT_SECRET`). It checks the token only: no admin role (`admin_role`) is enforced on any route, so every admin can call every admin endpoint.
- Error body: `{"code","message","request_id"}`; see `error-codes.md`.
- `RateLimit(limit, window)`: Redis fixed window keyed by client IP + route; fail-open on Redis errors; 429 `ERR_RATE_LIMITED`. `RateLimitUser` is the same keyed by user id.
- `RequireIdempotencyKey`: header `Idempotency-Key` required, at most 64 chars, else 400 `ERR_IDEMPOTENCY_KEY_REQUIRED`. Used on every mounted money-moving or money-object-creating POST (marked IDEM below). The key is scoped to the calling user and bound to the request payload: same key and payload returns the original result, same key and another payload is 409 `ERR_IDEMPOTENCY_KEY_REUSED` (ADR-0011).
- Every response carries `X-Request-Id` (echoed or generated).
- Money amounts are integers in VND minor units.

## Public / infrastructure

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| GET, HEAD | /healthz | public | none | healthz | Postgres and Redis readiness; 503 `degraded` with raw error text when a dependency fails |
| POST | /internal/mock-bank/callback | shared secret header (`MOCK_BANK_SECRET`), no JWT | none | MockBankHandler.Callback | Simulated bank top-up confirmation; mounted in every env; must be blocked at the proxy |

## Auth (`/v1/auth`)

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| POST | /v1/auth/register | public | RateLimit 5/min | AuthHandler.Register | Create user; ensures main wallet; returns tokens |
| POST | /v1/auth/login | public | RateLimit 5/min | AuthHandler.Login | Phone + password login; returns access + refresh |
| POST | /v1/auth/refresh | public (refresh token in body) | RateLimit 30/min | AuthHandler.Refresh | Rotate refresh token; reuse revokes the family |
| POST | /v1/auth/logout | public (refresh token in body) | none | AuthHandler.Logout | Revoke refresh family |

## Me, wallets, transactions (user JWT, role api/all)

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| GET | /v1/users/me | user | | UserHandler.Me | Profile, KYC status, txn limit and offer limits (from the user's offer) |
| GET | /v1/users/me/qr | user | | QRHandler.Mint | Signed receive-QR payload |
| POST | /v1/qr/resolve | user | RateLimitUser 30/min | QRHandler.Resolve | Verify QR, return receiver token |
| GET | /v1/users/lookup | user | RateLimitUser 20/min | UserHandler.Lookup | Find a user by phone |
| GET | /v1/wallets | user | | WalletHandler.List | Own wallets (main + vaults) |
| GET | /v1/transactions | user | | WalletHandler.History | Paged history |
| GET | /v1/transactions/:refCode | user | | WalletHandler.Detail | One transaction |
| GET | /v1/transactions/:refCode/timeline | user | | TransferHandler.Timeline | Workflow step timeline and terminal error code |
| GET | /v1/fees/preview | user | | TransferHandler.FeePreview | Fee for an amount |
| POST | /v1/transactions/request | user | IDEM | TransactionEngineHandler.RequestTransaction | Engine step 1: validate, OfferGuard, create the transaction for any `service_code`. When `auth_required` is `pin` or `otp` the transaction is held in `awaiting_auth` and nothing moves until step 2; `none` starts the workflow at once; a request needing `2fa` is refused (ERR_AUTH_METHOD_UNAVAILABLE) |
| POST | /v1/transactions/challenge | user | RateLimitUser 10/min | TransactionEngineHandler.ChallengeTransaction | Issue the OTP for a transaction in the otp band; the code is returned only when APP_ENV=dev |
| PUT | /v1/me/pin | user | RateLimitUser 10/min | PINHandler.Set | Set the first transaction PIN (account password step-up) |
| PATCH | /v1/me/pin | user | RateLimitUser 10/min | PINHandler.Change | Change the PIN (current PIN required; same lockout) |
| POST | /v1/transactions/confirm | user | RateLimitUser 20/min | TransactionEngineHandler.ConfirmTransaction | Engine step 2 (ADR-0014): authenticate with the server-verified PIN (plus OTP for the otp band) and start the recorded workflow once; unknown id and another user's id both return 404; repeated calls return the current state |
| POST | /v1/transactions/verify | user | none | TransactionEngineHandler.VerifyTransaction | Engine step 3 (2fa): ownership-checked; there is no 2fa provider, so it only reports state for the caller's own transaction |

## Vaults

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| GET | /v1/vaults | user | | VaultHandler.List | Own vaults |
| POST | /v1/vaults | user | | VaultHandler.Create | Create vault (label required, unique per user) |
| PATCH | /v1/vaults/:id/round-up-target | user | | VaultHandler.SetRoundUpTarget | Mark/unmark the round-up target vault |

Money in or out of a vault uses `POST /v1/transactions/request` with `service_code=VAULT_SAVINGS`.

## Money requests

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| POST | /v1/money-requests | user | IDEM | RequestHandler.Create | Ask one user for money |
| POST | /v1/money-requests/split | user | IDEM | RequestHandler.CreateSplit | Split a bill across payers |
| GET | /v1/money-requests/inbox | user | | RequestHandler.Inbox | Requests addressed to me |
| GET | /v1/money-requests/sent | user | | RequestHandler.Sent | Requests I created |
| GET | /v1/money-requests/:id | user | | RequestHandler.Detail | One request |
| POST | /v1/money-requests/:id/decline | user | | RequestHandler.Decline | Payer declines |
| POST | /v1/money-requests/:id/cancel | user | | RequestHandler.Cancel | Requester cancels |
| POST | /v1/money-requests/:id/remind | user | | RequestHandler.Remind | Nudge payer (cooldown) |

Paying a request is done through the transaction engine, there is no `/pay` route.

## KYC (user)

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| POST | /v1/kyc/submit | user | | KYCHandler.Submit | Multipart ID card + selfie (jpeg/png, max 5MB) |
| GET | /v1/kyc/status | user | | KYCHandler.Status | Current KYC state |

## Organization (`/v1/org`, user JWT, role admin/all)

Membership and role (maker/checker/admin) checks happen in the service layer, not in middleware.

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| GET | /v1/org | user | | OrgHandler.ListMyOrgs | Orgs I belong to |
| POST | /v1/org | user | | OrgHandler.Create | Create organization |
| POST | /v1/org/:id/members | user (org admin) | | OrgHandler.AddMember | Add member with role and checker level |
| POST | /v1/org/:id/policies | user (org admin) | | OrgHandler.CreatePolicy | New approval policy version |
| POST | /v1/org/:id/transactions | user (maker) | IDEM | OrgHandler.Disburse | Org disbursement; parks on approval |

## Approvals (org checker side, user JWT)

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| GET | /v1/org/approvals | user | | ApprovalHandler.Inbox | Approval inbox |
| GET | /v1/org/approvals/by-ref/:ref_code | user | | ApprovalHandler.DetailByRefCode | Detail by transaction ref code |
| GET | /v1/org/approvals/:id | user | | ApprovalHandler.Detail | Detail |
| POST | /v1/org/approvals/:id/challenge | user (checker) | RateLimit 10/min | ApprovalHandler.Challenge | Issue OTP; `otp_dev` returned only when `APP_ENV=dev` |
| POST | /v1/org/approvals/:id/decide | user (checker) | | ApprovalHandler.Decide | Approve or reject with OTP |
| POST | /v1/org/approvals/:id/cancel | user (maker) | | ApprovalHandler.Cancel | Maker cancels own pending request |

## Admin (`/v1/admin`, role admin/all)

Login is public; everything else requires `RequireAdmin` (admin JWT, no role check).

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| POST | /v1/admin/auth/login | public | RateLimit 5/min | AdminHandler.Login | Admin email + password |
| GET | /v1/admin/kyc | admin | | KYCQueue | KYC review queue |
| POST | /v1/admin/kyc/:id/review | admin | | KYCReview | Approve/reject KYC |
| GET | /v1/admin/fraud-flags | admin | | FraudFlags | Fraud flag list |
| POST | /v1/admin/fraud-flags/:id/resolve | admin | | ResolveFraudFlag | Clear or confirm fraud |
| POST | /v1/admin/reversals | admin | | CreateReversal | Request reversal (creates approval) |
| GET | /v1/admin/approvals | admin | | AdminApprovals | Reversal approval tasks |
| POST | /v1/admin/approvals/:id/decide | admin | | AdminDecide | Finalize reversal approval |
| POST | /v1/admin/recon/run | admin | | ReconRun | Run reconciliation for a date |
| GET | /v1/admin/recon/runs/:date | admin | | ReconGet | Run result and signed report link |
| GET | /v1/admin/services | admin | | ListServices | Service catalog |
| GET | /v1/admin/services/:id | admin | | GetService | One service |
| PUT | /v1/admin/services/:id | admin | | UpdateServiceStatus | Change service status |
| GET | /v1/admin/offers | admin | | ListOffers | Offers |
| GET | /v1/admin/offers/:id | admin | | GetOffer | Offer with services |
| GET | /v1/admin/workflows | admin | | ListWorkflows | Workflow instances |
| GET | /v1/admin/workflows/:id | admin | | GetWorkflow | Instance with steps |
| GET | /v1/admin/audit | admin | | ListAuditLogs | Audit log search |
| GET | /v1/admin/orgs | admin | | ListOrgs | All organizations |
| PUT | /v1/admin/orgs/:id/policies | admin | | UpdateOrgPolicy | Ops override of org policy |
| GET | /v1/admin/ai/keys | admin | | ListAIKeys | AI provider keys (masked) |
| POST | /v1/admin/ai/keys | admin | | CreateAIKey | Add key |
| PUT | /v1/admin/ai/keys/:id | admin | | UpdateAIKey | Update key |
| GET | /v1/admin/ai/settings | admin | | ListAISettings | Per-function AI settings |
| PUT | /v1/admin/ai/settings | admin | | UpdateAISetting | Set provider/model/key for a function |

## Files

| Method | Path | Auth | Middleware | Handler | Purpose |
|---|---|---|---|---|---|
| GET | /v1/files/*key | HMAC signature + expiry in query (5 minutes), no JWT | none | AdminHandler.ServeFile | Download KYC images and recon reports |

## Spec vs router mismatches

`python -X utf8 src/backend/scripts/check-openapi-sync.py` (run from `src/backend`, it opens relative paths) printed:

```
openapi sync OK: 68 operations match
```

It must be run with cwd `src/backend`; from the repo root it fails with FileNotFoundError. CI job `lint` runs it. Residual differences the script does not catch:

- `/healthz` is also served for HEAD (router) and documented only as GET.
- Spec describes `/v1/files/{key}`; router uses a catch-all `/v1/files/*key` (key may contain slashes).
- Handlers that exist without a route: `WalletHandler.Topup` and `TransferHandler.Transfer` (legacy; no route mounted, so no `/v1/wallets/topup` or `/v1/transactions/transfer`).
- Spec error codes not present in `errTable` (raised inline by handlers): `ERR_INVALID_CALLBACK_SECRET`, `ERR_KEY_NOT_FOUND`, `ERR_KEY_INACTIVE`, `ERR_RATE_LIMITED`, `ERR_IDEMPOTENCY_KEY_REQUIRED` (also in `errTable` since ADR-0011); spec lists `ERR_NOT_FOUND`, `ERR_SERVICE_NOT_FOUND`, `ERR_OFFER_NOT_FOUND`, `ERR_WORKFLOW_NOT_FOUND` and `ERR_FRAUD_BLOCKED`, `ERR_ACCOUNT_INACTIVE`, `ERR_TRANSFER_FAILED` (the latter three only as workflow terminal codes) which are not emitted by `respondErr`. See `error-codes.md`.

## Endpoints called by frontends that do not exist

Found by grepping `/v1/` in `src/admin/src` and `src/mobile/src` (excluding tests) and comparing with the router. These calls return 404 (or 405) today.

### Admin (`src/admin/src`)

| Call | File | Router reality |
|---|---|---|
| GET `/v1/admin/fraud/flags` | queries/useFraudQueries.ts:49 | Real route is `/v1/admin/fraud-flags` (the same file also calls it at :66) |
| POST `/v1/admin/fraud/flags/:id/resolve` | useFraudQueries.ts:102 | Real route is `/v1/admin/fraud-flags/:id/resolve` (used at :117) |
| POST `/v1/admin/kyc/:id/decision` | queries/useKYCQueries.ts:44 | Real route is `POST /v1/admin/kyc/:id/review` (used at :77) |
| PATCH `/v1/admin/services/:id/status` | queries/useServiceQueries.ts:60 | Real route is `PUT /v1/admin/services/:id` |
| POST `/v1/admin/services` (create, two hooks) | useServiceQueries.ts:90, :105 | No create route; only list/get/PUT |
| PUT or POST `/v1/admin/offers/:id` and POST `/v1/admin/offers/` | queries/useOffersQueries.ts:69 | Only GET list and GET one exist |

Some of these are duplicated hooks where one variant targets the real route; the dead variants are the ones listed.

### Mobile (`src/mobile/src`)

| Call | File | Router reality |
|---|---|---|
| POST `/v1/auth/verify-otp` | queries/useAuthQueries.ts:41 | No such route |
| GET `/v1/engine/status` | queries/useEngineQueries.ts:13 | No such route |
| POST `/v1/money-requests/:id/pay` | queries/useMoneyRequestQueries.ts:124 | No such route (payment goes through `/v1/transactions/request`) |
| POST `/v1/wallets/topup` | queries/useWalletQueries.ts:66 | Handler exists, route not mounted |
| POST `/v1/transactions/transfer` | queries/useWalletQueries.ts:84 | Handler exists, route not mounted |

Note: `schema.d.ts` in `src/admin/src/api` is stale; it still lists `/v1/wallets/topup`, `/v1/transactions/transfer`, `/v1/money-requests/{id}/pay`, `/v1/org/{id}/topup`, none of which exist in the current spec or router. The mobile `schema.d.ts` matches the current spec.
