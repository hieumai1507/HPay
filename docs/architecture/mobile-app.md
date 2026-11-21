---
title: Mobile app (src/mobile)
type: architecture
status: verified-against-code
last_verified: 2026-11-21
code: [src/mobile/package.json, src/mobile/app.config.ts, src/mobile/app, src/mobile/src/api/client.ts, src/mobile/src/api/refresh.ts, src/mobile/src/stores, src/mobile/src/lib/idempotency.ts, src/mobile/src/lib/intentKey.ts, src/mobile/src/lib/useSubmitGuard.ts, src/mobile/src/lib/availability.ts, src/mobile/src/features/security/usePin.tsx, src/mobile/src/features/security/unlockAttempts.ts, src/mobile/src/features/security/components/SecurityWrapper.tsx, src/mobile/src/queries, src/mobile/jest.config.js]
---

# Mobile app

## Tech stack (package.json)

Expo SDK 54, React Native 0.81.5, React 19.1, expo-router 6 (file-based routes), NativeWind 4 + Tailwind 3, TanStack Query 5, zustand 5, react-hook-form 7 + zod 4, `@shopify/flash-list`, reanimated 4, expo-secure-store, expo-local-authentication, expo-camera, expo-crypto (UUIDs), expo-screen-capture, expo-device, react-native-qrcode-svg. Tests: Jest 29 with `jest-expo` preset and `@testing-library/react-native` 13 (`jest.config.js`, `jest.setup.js`, mocks for `expo-crypto` and `expo-secure-store` in `__mocks__/`). Scripts: `start`, `android`, `ios`, `typecheck`, `lint`, `test`, `gen:api`. API base URL comes from `app.config.ts` `extra.apiBaseUrl`: `dev` uses the env var `EXPO_PUBLIC_API_URL` and defaults to `http://localhost:8080` (a physical device needs the LAN address of the dev machine); `staging`/`demo` are placeholders (see [deployment.md](deployment.md)). `ios.bundleIdentifier` and `android.package` are both `com.hieumai1507.hpay`.

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
| `security` | `SecurityWrapper` (privacy mask, idle lock, OS-verified unlock with 5-attempt sign-out), `usePin` + `PinPrompt` (PIN collection seam), `unlockAttempts`, `useSecureScreen` (screen capture prevention on sensitive screens), tests |
| `wallet` | `recent-contacts.ts`, test |
| `money-request` | test only (logic is in `queries/useMoneyRequestQueries.ts` and `lib/split.ts`) |

Other source folders: `src/api` (client, refresh, query-client, generated `schema.d.ts`), `src/queries` (one hook file per area: auth, engine, kyc, money-request, org, transaction, vault, wallet), `src/stores` (session, device, security, settings, org), `src/lib` (errors, idempotency, money, polling, split), `src/ui` (design-system components and `tokens.ts`).

## API client

`src/api/client.ts`: `apiRequest<T>(path, {method, body, idempotencyKey, skipAuth, headers})`. Every request carries `X-Request-Id` (random UUID), `X-Device-Id` (persisted device id) and, unless `skipAuth`, `Authorization: Bearer <in-memory access token>`. `stringifyWithBigInt` serializes `bigint` as raw JSON integers. Network failures throw `NetworkError`; non-2xx throw `ApiError(code, status, request_id, message)` from the backend envelope. On a 401 for an authenticated request it calls the registered unauthorized handler (single-flight `refreshAccessToken`) and retries exactly once. Query client: GET queries retry up to 2 times except for 4xx; mutations never retry (`query-client.ts`).

## Tokens and secure storage

- Access token: in memory only (`session.ts` `accessToken`), never persisted.
- Refresh token: `expo-secure-store` key `hpay_refresh_token` (`session.ts:8,54-64`). Refresh rotates the token and stores the new one; `ERR_TOKEN_REUSED` maps to logout reason `refresh_revoked`, other failures to `refresh_expired` (`refresh.ts:19-53`). Only HTTP 401/403 from `/v1/auth/refresh` clears the stored token and the session. Network errors, 5xx and 429 keep both and surface a retryable error (`refresh.ts`, test `TestEdge_M04_Refresh_TransientFailure_KeepsSession`); at app start a transient failure leaves the bootstrapping state without deleting the token, so the next launch retries.
- Device id: random UUID in SecureStore key `hpay_device_id` (`stores/device.ts`).
- KYC photo draft: only the local file URIs (short strings, about 100-200 characters) are stored in SecureStore (`mp_kyc_draft_*`), never image bytes, so the per-item size limit is not a concern. The URIs point at files in the app cache, which the OS may purge; the screens then simply ask for a new photo.
- Intent idempotency keys: map `intent -> {key, at}` in SecureStore key `hpay_intent_keys` (`lib/intentKey.ts`), 30 minute TTL.
- Balance visibility is not persisted and starts hidden (`stores/settings.ts`).
- Logout calls `POST /v1/auth/logout` and ignores network errors (`account.tsx:113-117`).

## Idempotency and submit guard

- One Idempotency-Key per user intent, persisted (`lib/intentKey.ts`): `withIntentKey(intent, run)` returns the same key for the same intent across retries, re-renders, remounts of the confirm screen and app restarts (SecureStore, 30 min TTL). The key is released on success and on a definitive 4xx rejection (not 409/429); on network errors, 5xx, 409 and 429 it is kept so the retry replays the same key. Intent strings: `transfer:<receiver token or phone>:<amount>:<note>`, `vault:<id>:<mode>:<amount>`, `disburse:<org>:<phone>:<amount>:<note>`, `policy:<org>:<tiers>`, `topup:<amount>`, `pay:<request id>`. The old `useIdempotencyKey` (a `useMemo` per mount) remains only for request create and split (`requests/create.tsx`, `split.tsx`), which create a money request but do not move money.
- Double submit: `useSubmitGuard()` (`lib/useSubmitGuard.ts`) has a synchronous ref flag plus `isPending` state; `run(action)` returns `undefined` when swallowed, releases the guard in `finally`. Used on transfer confirm, vault move, vault create, org disburse, org policy save, top-up and pay request. Before, `transfer/confirm.tsx` guarded on `useTransfer().isPending` (a mutation that is never called), so the guard never tripped.
- The backend only enforces the header on `/transactions/request` and `/org/:id/transactions`.

## Money handling

Amounts are `bigint` from input to wire. `parseAmountStrict` (`lib/money.ts`) parses user input and rejects empty, zero and anything above `MAX_AMOUNT` (1,000,000,000 VND, the largest backend per-transaction cap); `percentOf` does progress percentages in bigint; `EngineRequestPayload.amount`, vault, org disbursement and policy tier amounts are `bigint` and go through `stringifyWithBigInt`. Guards: ESLint `no-restricted-syntax` in `eslint.config.js` and the jest test `TestEdge_C01_NoFloatMoney` fail on `Number()/parseInt()/parseFloat()` applied to identifiers or members named amount/balance/fee/total/target.

## PIN

- Hard-coded PINs are gone (jest test `TestEdge_C07_NoHardCodedPin` fails if a `'123456'` literal returns).
- `usePin()` (`features/security/usePin.tsx`) is the single seam: `const pin = await requestPin()` shows `PinPrompt` (6 digits, `OTPInput`) and resolves with the digits or `null` on cancel. Money flows pass the value as `pin` to `POST /v1/transactions/confirm` (transfer confirm, vault move). Nothing is validated or stored client-side.
- STUBBED: server-side PIN verification. The backend ignores `pin` (audit BE-C2, ADR-0008 pending). Until then the PIN prompt only collects a value; it does not authenticate. There is no PIN set-up screen and no "wrong PIN" handling.
- App lock (`SecurityWrapper.tsx`) no longer compares anything with a literal. It unlocks only through `LocalAuthentication.authenticateAsync` (biometrics or the device credential, verified by the OS). `features/security/unlockAttempts.ts` counts failures: user cancel is not counted, the 5th failure (`MAX_UNLOCK_ATTEMPTS`) clears the stored refresh token and signs out. A device with no biometric and no screen lock cannot unlock and is told to sign out. `// TODO(ADR-0008)` marks where server verification goes. The lock is a device-level gate, not authentication of the account holder against HPay.
- The `biometricEnabled` toggle in the account tab no longer gates unlock (the lock always uses OS authentication).
## Other security behaviour

- `useSecureScreen` blocks screenshots on sensitive screens (vault detail uses it).
- Background/inactive shows a privacy mask; after 5 minutes in background the app locks (`IDLE_TIMEOUT_MS`). Root detection is a stub that only flags `Device.brand === 'mock-rooted'`.

## Endpoints called vs backend

Matches the router: auth login/register/refresh/logout, `/users/me`, `/users/me/qr`, `/qr/resolve`, `/users/lookup`, `/wallets`, `/transactions` (+detail, timeline), `/fees/preview`, `/transactions/request|confirm`, `/vaults` (+round-up-target), `/money-requests` (+split, inbox, sent, `:id`, decline, cancel, remind), `/kyc/submit|status`, `/org` (list, create), `/org/:id/members|policies|transactions`, `/org/approvals` (inbox, `:id`, `by-ref`, challenge, decide, cancel).

Called but NOT routed in the backend (audit appendix) and what the app does now:

| Call | Decision |
|---|---|
| `POST /v1/auth/verify-otp` | The dead hook `useVerifyOtp` was removed. The register OTP screen never sent its code anywhere (`otp.tsx` just calls `/v1/auth/register`); that is a backend gap (no OTP verification exists), not changed here. |
| `GET /v1/engine/status` | Dead hook `useEngineStatus` removed (no screen used it). |
| `POST /v1/wallets/topup` | Backend gap. The engine service `TOPUP_BANK` credits from SYSTEM_CASH_IN with no bank step, so it is not an equivalent. `topup/amount.tsx` shows a "not available yet" state behind `FEATURE_AVAILABILITY.topup` (`lib/availability.ts`); the form and `useTopup` stay for when the route is mounted. |
| `POST /v1/money-requests/:id/pay` | Backend gap. Paying through the engine (`MONEY_REQUEST`) moves money but would not mark the request paid, so it is not an equivalent. `requests/pay.tsx` shows "not available yet" behind `FEATURE_AVAILABILITY.payRequest`. The unused engine helpers `payMoneyRequest`, `topupBank`, `disburseOrg` were removed. |
| `POST /v1/transactions/transfer` | Equivalent exists and is already used: transfer confirm uses `/v1/transactions/request` then `/confirm`. The dead `useTransfer` hook was removed. |
## Test setup

`npm test` = `jest` with `jest-expo`; `moduleNameMapper` maps `@/` to the project root. Test files: `src/api/__tests__/refresh.test.ts`, `__tests__/app-config.test.ts`, `src/lib/__tests__/{errors,idempotency,intentKey,money,money-bounds,no-float-money,polling,split,useSubmitGuard}.test.ts`, `src/features/{kyc,money-request,security,wallet}/__tests__` (security: `unlock-attempts`, `usePin`), `src/stores/__tests__/org.test.ts`, `src/ui/__tests__/*`, `app/(main)/org/__tests__/order-status.test.tsx`. No CI workflow runs them (see [deployment.md](deployment.md)); `typecheck` and `lint` are manual.
