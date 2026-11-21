---
title: Mobile app (src/mobile)
type: architecture
status: verified-against-code
last_verified: 2026-11-21
code: [src/mobile/package.json, src/mobile/app.config.ts, src/mobile/app, src/mobile/src/api/client.ts, src/mobile/src/api/refresh.ts, src/mobile/src/stores, src/mobile/src/lib/idempotency.ts, src/mobile/src/features/security/components/SecurityWrapper.tsx, src/mobile/src/queries, src/mobile/jest.config.js]
---

# Mobile app

## Tech stack (package.json)

Expo SDK 54, React Native 0.81.5, React 19.1, expo-router 6 (file-based routes), NativeWind 4 + Tailwind 3, TanStack Query 5, zustand 5, react-hook-form 7 + zod 4, `@shopify/flash-list`, reanimated 4, expo-secure-store, expo-local-authentication, expo-camera, expo-crypto (UUIDs), expo-screen-capture, expo-device, react-native-qrcode-svg. Tests: Jest 29 with `jest-expo` preset and `@testing-library/react-native` 13 (`jest.config.js`, `jest.setup.js`, mocks for `expo-crypto` and `expo-secure-store` in `__mocks__/`). Scripts: `start`, `android`, `ios`, `typecheck`, `lint`, `test`, `gen:api`. API base URL comes from `app.config.ts` `extra.apiBaseUrl` (dev points to a remote host today, see [deployment.md](deployment.md)).

## Route map (`app/`)

Root `app/_layout.tsx` runs `useSessionBootstrap()` (device id + silent refresh), holds the splash screen, wraps the stack in `QueryClientProvider` and `SecurityWrapper`. `app/index.tsx` is the splash gate: authenticated and `kyc_status === 'none'` and not skipped goes to `/(kyc)/intro`, authenticated otherwise `/(main)/home`, unauthenticated `/(auth)/onboarding`. There is no route-level auth guard beyond this redirect: expo-router groups are not protected, authentication is enforced by the API returning 401.

| Group | Screen file | Purpose |
|---|---|---|
| `(auth)` | `onboarding.tsx`, `login.tsx`, `register.tsx`, `otp.tsx`, `_layout.tsx` | intro slides, login, registration form, OTP-style screen |
| `(kyc)` | `intro.tsx`, `capture-id.tsx`, `capture-selfie.tsx`, `status.tsx`, `_layout.tsx` | KYC flow |
| `(main)` tabs | `home.tsx`, `history.tsx`, `qr.tsx`, `account.tsx` | the 4 bottom tabs; `txn-detail.tsx` is a hidden tab screen |
| `(main)/topup` | `amount.tsx`, `result.tsx` | bank top-up |
| `(main)/transfer` | `receiver.tsx`, `amount.tsx`, `confirm.tsx`, `processing.tsx`, `result.tsx` | P2P transfer |
| `(main)/requests` | `index.tsx`, `create.tsx`, `split.tsx`, `detail.tsx`, `pay.tsx` | money requests and split bill |
| `(main)/vaults` | `index.tsx`, `[id].tsx` | savings vaults, deposit/withdraw, round-up target |
| `(main)/org` | `admin.tsx`, `disburse.tsx`, `approvals.tsx`, `approval-detail.tsx`, `order-status.tsx` (+ `__tests__/order-status.test.tsx`) | business wallet: members/policy, disbursement, approvals |

Home quick actions: top-up, transfer, QR, requests, vaults (`home.tsx`). The account tab lists the user's orgs and lets the user switch to an org context (`useOrgStore.switchOrg`), toggles biometric lock and logs out.

## Feature modules (`src/features`)

| Module | Content |
|---|---|
| `auth` | `OTPInput` component, zod `auth-validation` schemas |
| `kyc` | `KYCPhotoFrame`, `photo-validator.ts` (heuristic: URIs containing `blurry`/`dark` are rejected, intended for tests), `kyc-draft` store (drafts stored in SecureStore), tests |
| `security` | `SecurityWrapper` (privacy mask, idle lock, PIN pad), `useSecureScreen` (screen capture prevention on sensitive screens), tests |
| `wallet` | `recent-contacts.ts`, test |
| `money-request` | test only (logic is in `queries/useMoneyRequestQueries.ts` and `lib/split.ts`) |

Other source folders: `src/api` (client, refresh, query-client, generated `schema.d.ts`), `src/queries` (one hook file per area: auth, engine, kyc, money-request, org, transaction, vault, wallet), `src/stores` (session, device, security, settings, org), `src/lib` (errors, idempotency, money, polling, split), `src/ui` (design-system components and `tokens.ts`).

## API client

`src/api/client.ts`: `apiRequest<T>(path, {method, body, idempotencyKey, skipAuth, headers})`. Every request carries `X-Request-Id` (random UUID), `X-Device-Id` (persisted device id) and, unless `skipAuth`, `Authorization: Bearer <in-memory access token>`. `stringifyWithBigInt` serializes `bigint` as raw JSON integers. Network failures throw `NetworkError`; non-2xx throw `ApiError(code, status, request_id, message)` from the backend envelope. On a 401 for an authenticated request it calls the registered unauthorized handler (single-flight `refreshAccessToken`) and retries exactly once. Query client: GET queries retry up to 2 times except for 4xx; mutations never retry (`query-client.ts`).

## Tokens and secure storage

- Access token: in memory only (`session.ts` `accessToken`), never persisted.
- Refresh token: `expo-secure-store` key `hpay_refresh_token` (`session.ts:8,54-64`). Refresh rotates the token and stores the new one; `ERR_TOKEN_REUSED` maps to logout reason `refresh_revoked`, other failures to `refresh_expired` (`refresh.ts:19-53`). Note any refresh failure (including network-level ones that surface as ApiError) clears the stored token and the session (audit mobile finding: should only clear on 401/403).
- Device id: random UUID in SecureStore key `hpay_device_id` (`stores/device.ts`).
- KYC photo draft URIs in SecureStore (`mp_kyc_draft_*`).
- Balance visibility is not persisted and starts hidden (`stores/settings.ts`).
- Logout calls `POST /v1/auth/logout` and ignores network errors (`account.tsx:113-117`).

## Idempotency usage

`useIdempotencyKey(resetKey)` memoizes one UUID per flow so a double tap reuses the key (`lib/idempotency.ts:21`). Used on: transfer confirm (`transfer/confirm.tsx:30`), top-up amount (`topup/amount.tsx:22`), request create/split/pay (`requests/create.tsx:22`, `pay.tsx:22`, `split.tsx`), vault move (`vaults/[id].tsx:32`). Org disbursement and org policy creation use `generateIdempotencyKey()` at call time (`org/disburse.tsx:56`, `org/admin.tsx:151`), i.e. a fresh key per tap, so a retry after timeout is not deduplicated. The backend only enforces the header on `/transactions/request` and `/org/:id/transactions`.

## PIN and lock handling as implemented

- There is no PIN setup screen and no server PIN. Every money flow sends the literal `pin: '123456'` to `POST /v1/transactions/confirm`: transfer confirm (`transfer/confirm.tsx:53`), money-request pay (`useTransactionQueries.ts:155,166,179`), vault moves (`useVaultQueries.ts:81`). The backend ignores it (see [crosscutting-concepts.md](crosscutting-concepts.md), BE-C2). Audit lists this as Critical.
- App lock (`SecurityWrapper.tsx`): after 5 minutes in background the app locks (`IDLE_TIMEOUT_MS`, line 14); unlock by 6-digit PIN compared to the constant `'123456'` in the client (lines 108-126) or by biometrics when the user enabled it (`biometricEnabled` defaults to false). Background/inactive shows a privacy mask. Root detection is a stub that only flags `Device.brand === 'mock-rooted'`. No attempt limit on the PIN pad.
- `useSecureScreen` blocks screenshots on sensitive screens (vault detail uses it).

## Endpoints called vs backend

Matches the router: auth login/register/refresh/logout, `/users/me`, `/users/me/qr`, `/qr/resolve`, `/users/lookup`, `/wallets`, `/transactions` (+detail, timeline), `/fees/preview`, `/transactions/request|confirm`, `/vaults` (+round-up-target), `/money-requests` (+split, inbox, sent, `:id`, decline, cancel, remind), `/kyc/submit|status`, `/org` (list, create), `/org/:id/members|policies|transactions`, `/org/approvals` (inbox, `:id`, `by-ref`, challenge, decide, cancel).

Called but NOT routed in the backend (audit appendix, re-verified): `POST /v1/auth/verify-otp` (`useAuthQueries.ts:41`; the register OTP screen never sends its code anywhere, it just calls `/v1/auth/register`, `otp.tsx:62-80`), `GET /v1/engine/status` (`useEngineQueries.ts:13`), `POST /v1/money-requests/:id/pay` (`useMoneyRequestQueries.ts:124`, used by `usePayRequest` in `requests/pay.tsx`; `useTransactionEngine.payMoneyRequest` is the alternative path through the engine with `MONEY_REQUEST` service), `POST /v1/wallets/topup` (`useWalletQueries.ts:66`, used by `topup/amount.tsx`), `POST /v1/transactions/transfer` (`useWalletQueries.ts:84`, `useTransfer`, only its `isPending` is read in `transfer/confirm.tsx`; the actual transfer uses the engine). So the top-up screen and the "pay request" screen call endpoints that return 404 today.

## Test setup

`npm test` = `jest` with `jest-expo`; `moduleNameMapper` maps `@/` to the project root. Test files: `src/api/__tests__/refresh.test.ts`, `src/lib/__tests__/{errors,idempotency,money,polling,split}.test.ts`, `src/features/{kyc,money-request,security,wallet}/__tests__`, `src/stores/__tests__/org.test.ts`, `src/ui/__tests__/*`, `app/(main)/org/__tests__/order-status.test.tsx`. No CI workflow runs them (see [deployment.md](deployment.md)); `typecheck` and `lint` are manual.
