---
title: Feature catalog
type: product
status: verified-against-code
last_verified: 2026-11-21
code: [src/backend/internal/api/router.go, src/mobile/app, src/mobile/src/queries, src/admin/src/router.tsx, src/admin/src/queries]
---

# Feature catalog

Status legend: **implemented** = UI (where applicable) and a mounted backend route both exist and are consistent; **partial** = works with a known defect or one side is missing; **mock-only** = UI shows invented data or the call hits a missing endpoint. Cross-checked on 2026-11-21 between mobile screens/hooks, admin pages/hooks and `router.go`. Roles: see [roles-and-permissions.md](roles-and-permissions.md). Defect IDs: [../audit/AUDIT-REPORT.md](../audit/AUDIT-REPORT.md).

## A. Personal user (mobile)

| # | Feature | What it does | Screens | Endpoints | Status |
|---|---|---|---|---|---|
| A1 | Register | phone + password + name, creates user, main wallet, returns tokens | `(auth)/register`, `otp` | `POST /v1/auth/register` | partial: the OTP screen collects a code that is never sent or verified (`otp.tsx:62-80`); `useVerifyOtp` targets nonexistent `/v1/auth/verify-otp` |
| A2 | Login / refresh / logout | argon2id password check, rotating refresh with theft detection, device id | `(auth)/login`, app bootstrap, account tab | `POST /v1/auth/login|refresh|logout` | implemented (rate limited; no per-account lockout, BE-H8) |
| A3 | Profile and limit | name, KYC status, `txn_limit` | account tab, top-up | `GET /v1/users/me` | implemented |
| A4 | Wallet balance and list | main wallet and vault wallets; balance hidden by default | home | `GET /v1/wallets` | implemented |
| A5 | Transaction history, detail, timeline | cursor pagination, filter chips, workflow timeline | `history`, `txn-detail` | `GET /v1/transactions`, `/:refCode`, `/:refCode/timeline` | implemented |
| A6 | Receiver lookup | masked name by phone, rate limited | `transfer/receiver` | `GET /v1/users/lookup` | implemented |
| A7 | Fee preview | fee and total for an amount | `transfer/confirm`, `requests/pay` | `GET /v1/fees/preview` | implemented |
| A8 | P2P transfer | request then confirm through the workflow engine; result polling | `transfer/receiver`, `amount`, `confirm`, `processing`, `result` | `POST /v1/transactions/request` (Idempotency-Key), `/confirm` | partial: money moves at `request`; the app now asks the user for a PIN (`usePin`) and sends it, but the server ignores it (BE-C2, ADR-0008 pending); one persisted idempotency key per transfer intent and a single-flight submit guard; double-post window in MOVE_FUNDS (BE-C1); OfferGuard limits apply, `TxnLimit` does not |
| A9 | QR code | mint own signed QR (15 min), scan and resolve to receiver token | `qr` tab | `GET /v1/users/me/qr`, `POST /v1/qr/resolve` | implemented |
| A10 | Bank top-up | amount screen and result | `topup/amount`, `result` | app would call `POST /v1/wallets/topup` which is NOT mounted; the screen is gated (`FEATURE_AVAILABILITY.topup = false`) | **not available** (screen shows a not-available state, no call is made; fixed in working tree 2026-11-21, not yet committed). Backend side: mock-bank callback `POST /internal/mock-bank/callback` completes pending top-ups created by seeders/tests; engine service `TOPUP_BANK` exists |
| A11 | Money request | ask a phone number for money; inbox, sent, detail, decline, cancel (requester), remind (rate limited), expiry 72 h by sweeper | `requests/index`, `create`, `detail` | `POST /v1/money-requests`, `GET .../inbox|sent|:id`, `POST .../:id/decline|cancel|remind` | implemented (no idempotency header enforced, request marked paid before funds move BE-H5) |
| A12 | Pay a money request | payer pays and funds move | `requests/pay` | app would call `POST /v1/money-requests/:id/pay` (not mounted); the screen is gated (`FEATURE_AVAILABILITY.payRequest = false`); `Service.Pay` exists in backend; the engine `MONEY_REQUEST` service moves money but does not mark the request paid, so it is not used | **not available** (not-available state, no call is made; fixed in working tree 2026-11-21, not yet committed) |
| A13 | Split bill | split total among phones equally or custom amounts | `requests/split` | `POST /v1/money-requests/split` | implemented |
| A14 | Vaults (savings pockets) | create, list, deposit/withdraw via engine service `VAULT_SAVINGS`, round-up target (one per user) | `vaults/index`, `[id]` | `GET/POST /v1/vaults`, `PATCH /v1/vaults/:id/round-up-target`, `POST /v1/transactions/request|confirm` | implemented (round-up step runs after P2P CONFIRM in `p2p_transfer` v3) |
| A15 | KYC | capture ID and selfie, submit, status; AI score auto-approve/manual review/reject | `(kyc)/intro`, `capture-id`, `capture-selfie`, `status` | `POST /v1/kyc/submit`, `GET /v1/kyc/status` | implemented; client photo validator is a filename heuristic; camera mock buttons are shown when no camera permission; submit uses raw `fetch` multipart (`capture-selfie.tsx:106`) |
| A16 | App lock and privacy | 5-min idle lock, OS biometric/device-credential unlock with sign-out after 5 failed attempts, privacy mask | `SecurityWrapper` | none | partial: no client PIN any more; no server verification yet (TODO ADR-0008), so the lock is a device-level gate only |
| A17 | Org context switch | switch the app into a business wallet | account tab | `GET /v1/org` | implemented |
| A18 | Forgotten PIN reset | backend only: password plus one-time code, then a new PIN; operator clear with `pin.reset`; 24 h cooling cap | none yet (no mobile screen) | `POST /v1/me/pin/reset/request`, `POST /v1/me/pin/reset/confirm`, `POST /v1/admin/users/:id/pin/reset` | partial: backend implemented (ADR-0016); no mobile or admin screen, no real SMS or e-mail provider |

## B. Business user (mobile, org context)

Roles admin, maker, checker: see roles doc.

| # | Feature | What it does | Screens | Endpoints | Status |
|---|---|---|---|---|---|
| B1 | Create organization | creates org, org wallet, creator is admin | account tab | `POST /v1/org` | implemented |
| B2 | Manage members | add a member by phone with role and checker level | `org/admin` | `POST /v1/org/:id/members` | implemented (admin only) |
| B3 | Approval policies | multi-tier amount ranges, k signatures per level, optimistic version | `org/admin` | `POST /v1/org/:id/policies` (user), `PUT /v1/admin/orgs/:id/policies` (admin web) | implemented (policy idempotency key generated per tap) |
| B4 | Disbursement (maker) | pay a receiver from the org wallet; waits for approval | `org/disburse`, `order-status` | `POST /v1/org/:id/transactions` | implemented |
| B5 | Approvals inbox (checker) | pending requests for my level, count on home | `org/approvals`, home | `GET /v1/org/approvals` | implemented |
| B6 | Approve / reject with OTP | challenge issues OTP (shown as `otp_dev` only in dev), decide with OTP | `org/approval-detail` | `POST /v1/org/approvals/:id/challenge|decide`, `GET .../:id`, `.../by-ref/:ref` | partial: in non-dev there is no OTP delivery channel (no SMS provider in repo) so a checker cannot obtain the code |
| B7 | Cancel own pending order (maker) | withdraws request | `order-status` | `POST /v1/org/approvals/:id/cancel` | implemented |
| B8 | Request expiry | overdue approvals expire (24 h default) | none (sweeper) | background | implemented |
| B9 | Org wallet top-up | service method exists, no route, no screen | - | - | not exposed |

## C. Admin / ops (web back-office)

All use `/v1/admin/*` with an admin JWT; backend does not check roles (see roles doc).

| # | Feature | What it does | Page | Endpoints | Status |
|---|---|---|---|---|---|
| C1 | Admin login | email + password, 15-min token, no refresh | `/login` | `POST /v1/admin/auth/login` | implemented |
| C2 | Dashboard | KYC and fraud counts plus KPI cards and hourly chart | `/dashboard` | `GET /v1/admin/kyc`, `/fraud-flags` | partial: no analytics API exists, so KPI cards and the hourly chart show an em dash in production; demo numbers only with `VITE_USE_MOCKS=true` in a dev server; a failed KYC/fraud count shows an em dash and an error hint |
| C3 | KYC review | queue by tab, detail with signed image links, approve/reject with reason | `/kyc`, `/kyc/:id` | `GET /v1/admin/kyc`, `POST /v1/admin/kyc/:id/review` | implemented |
| C4 | Fraud monitor | list flags (auto refresh 30 s), resolve as clear or confirmed fraud with reason; rules VELOCITY_5MIN, AMOUNT_SPIKE, ODD_HOURS, NEW_RECEIVER, flag >= 40, block > 70, optional LLM advisory text | `/fraud` | `GET /v1/admin/fraud-flags`, `POST .../:id/resolve` | partial: the flags API returns no amount or user, so those columns show an em dash (nothing is invented any more); resolve buttons need `fraud.resolve` (UX only) and disable while pending; each click sends an Idempotency-Key (the backend ignores it today) |
| C17 | Needs attention queue (G12) | read-only list of approvals parked as `needs_manual` and workflows in `compensation_failed` with ref code, amount, age, reason, detail link; sidebar count badge | `/attention` | `GET /v1/admin/approvals?status=needs_manual`, `GET /v1/admin/workflows?status=compensation_failed`, `GET /v1/admin/attention/summary` | implemented (read only; `workflow.action`; approval rows link to the audit log, no approval detail page) |
| C5 | Workflow monitor | instances with stuck detection, detail with step timeline | `/workflows`, `/workflows/:id` | `GET /v1/admin/workflows`, `/:id` | implemented (read only; no retry/force action endpoint) |
| C6 | My Tasks (admin approvals) | pending admin approvals, e.g. reversals, with decision reason | `/tasks` | `GET /v1/admin/approvals`, `POST .../:id/decide` | implemented |
| C7 | Create reversal | request reversal of a completed transaction by ref code, goes to admin approval | none | `POST /v1/admin/reversals` | backend only, no UI |
| C8 | Reconciliation | run for a date, view matched/mismatched items, AI summary, xlsx report link; nightly cron 00:15 | `/recon` | `POST /v1/admin/recon/run`, `GET /v1/admin/recon/runs/:date` | implemented in the UI: a failed call shows the real error, missing numbers show an em dash, mock run only with `VITE_USE_MOCKS=true` in dev; run button needs `recon.run` (UX only), disables while pending, sends an Idempotency-Key |
| C9 | Service catalog | list services, change status; create service in UI | `/services` | `GET /v1/admin/services`, `GET /:id`, `PUT /:id` | partial: status change works (needs `admins.manage`, UX only); **create is disabled with a visible reason, backend gap: no `POST /v1/admin/services`**; the pipeline modal's save is disabled (no endpoint) |
| C10 | Offer factory | list offers with services, view detail | `/offers`, `/offers/:id` | `GET /v1/admin/offers`, `/:id` | partial: read only; **save is disabled with a visible reason, backend gap: no `PUT/POST /v1/admin/offers/:id`**; limit inputs are validated integers (digits only, no negatives, upper bound); no fabricated services when an offer has none |
| C11 | Orgs and policies | list orgs, edit policy | `/orgs` | `GET /v1/admin/orgs`, `PUT /v1/admin/orgs/:id/policies` | implemented in the UI: errors are shown (no mock fallback in production); policy edit needs `orgs.edit` (UX only), disables while pending, amounts use integer math and the saved version number comes from the API |
| C12 | AI settings | manage provider keys, choose provider per purpose (kyc scoring etc.), mock engine | `/ai-settings` | `GET/POST /v1/admin/ai/keys`, `PUT .../keys/:id`, `GET/PUT /v1/admin/ai/settings` | partial: errors are shown and a failed write is a failure (no fake success, no fabricated keys); writes need `ai.manage` (UX only); keys stored in plaintext on the backend |
| C13 | Audit log | search and page audit events | `/audit` | `GET /v1/admin/audit` | implemented; errors are shown (demo rows only with `VITE_USE_MOCKS=true` in dev) |
| C14 | Signed file access | KYC images and recon reports through 5-min HMAC links | within C3, C8 | `GET /v1/files/*key` | implemented |
| C16 | Route guards (all `/orgs`, `/ai-settings`, `/services`, `/offers`, `/recon`, `/audit`, `/fraud`) | `RequirePermission` hides pages the role cannot use; sidebar hides the same links | all | none (UX only, BE-H6) | implemented in the UI; **not a security control** |
| C15 | Admin user management | `admins.manage` permission exists in the UI map | none | none | not present |

## D. Platform capabilities (no direct UI)

| Capability | Where | Status |
|---|---|---|
| Double-entry ledger, append-only | `ledger`, migration 000007 | implemented (BE-C1 caveat) |
| Workflow engine with retry, compensation, recovery | `workflow` | implemented (compensation failure leaves state unresolved, audit) |
| Fee policies by type and KYC tier | `fee` | implemented; only a 1,000 VND p2p fee seeded |
| OfferGuard limits and auth matrix | `offerguard` | implemented (race BE-H2) |
| Action pipeline (3rd-party actions per service) | `workflow/handlers/action_pipeline.go` | implemented; real HTTP only in `production`, SSRF risk |
| Fraud rule engine step | `fraud` | implemented |
| Reversal executor | `reversal` | implemented (via admin approval; double-reversal DB guard missing BE-H3) |
| Mock bank callback and statements | `wallet/callback.go` | implemented |
| OpenAPI contract | `api/openapi.yaml`, CI sync check | implemented (frontends drift, see audit appendix) |

## Cross-check summary

Mobile hooks calling routes that do not exist: `/v1/money-requests/:id/pay` and `/v1/wallets/topup` (both now behind `FEATURE_AVAILABILITY` flags, not called); the dead hooks for `/v1/auth/verify-otp`, `/v1/engine/status` and `/v1/transactions/transfer` were removed from the app (2026-11-21, working tree). Admin hooks (used by pages) calling routes that do not exist: `POST /v1/admin/services`, `PUT|POST /v1/admin/offers/:id`. Backend routes with no client: `POST /v1/transactions/verify`, `POST /v1/admin/reversals`, `GET /v1/transactions/:refCode/timeline` is used by mobile only. Unmounted handler code: `WalletHandler.Topup`, `TransferHandler.Transfer`, `Service.Pay`, `Service.Topup` for org.
