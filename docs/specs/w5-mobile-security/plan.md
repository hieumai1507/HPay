---
title: Mobile screens for PIN, forgotten PIN and TOTP
type: plan
status: proposed
last_verified: 2026-11-24
adr: ../../adr/0016-pin-reset.md, ../../adr/0017-totp-second-factor.md
---

# Plan: mobile security flows (PLAN G6, part of G9/G12 follow-ups)

The backend for PIN set, PIN reset (ADR-0016) and TOTP (ADR-0017) is done and tested. The mobile app cannot use it yet. The owner will redesign the UI soon, so this plan builds behaviour on existing components and keeps markup plain: no new visual design, no new tokens. A redesign later only restyles.

## What is missing today (read from src/mobile, 2026-11-24)
| # | Gap | Evidence |
|---|-----|----------|
| M1 | No screen sets the first PIN, so a new user cannot pass the PIN step of any payment | no PIN route under `app/`; `usePin` only collects digits (its comment still says the server PIN does not exist: stale) |
| M2 | No forgotten-PIN flow | `POST /v1/me/pin/reset/{request,confirm}` unused |
| M3 | No TOTP enrolment, no recovery-code screen, no TOTP entry at confirm | `confirmTransaction` payload has `pin` and `otp` only (`useTransactionQueries.ts:102`); no `totp` |
| M4 | The otp and 2fa bands cannot be confirmed from the transfer screen | `transfer/confirm.tsx` always asks for a PIN and sends nothing else; no `challenge` call |
| M5 | Error copy for new backend codes is missing | `src/lib/errors.ts` has none of ERR_PIN_COOLING, ERR_WEAK_PIN, ERR_PIN_NOT_SET, ERR_TOTP_*, ERR_LIMIT_DAILY, ERR_LIMIT_MONTHLY, ERR_TOO_FREQUENT, ERR_AUTH_EXPIRED |
| M6 | Generated API types are stale | `src/api/schema.d.ts` predates the PIN/TOTP routes |

## Scope and order (each step is one commit, tests first)
1. **M6 + M5**: regenerate `schema.d.ts` from `src/backend/api/openapi.yaml` (use the project's script; check `package.json`), add the missing error codes with Vietnamese copy, a unit test per code. Update the stale `usePin` comment.
2. **M1 Set PIN** (`app/(main)/security/set-pin.tsx`): password + new PIN + confirm PIN, client-side weak-PIN hints that mirror the server (6 digits, not all the same, no run) but the server stays the authority (`ERR_WEAK_PIN`). Reached from account settings and, when a payment needs a PIN and none is set (`ERR_PIN_NOT_SET`), from the confirm screen.
3. **M2 Forgot PIN** (`security/reset-pin.tsx`): step 1 password, step 2 code + new PIN. Show the 30 s resend cooldown (`ERR_OTP_COOLDOWN`), 3-attempt exhaustion, the dev code only when the API returns `otp_dev`. After success explain the 24 h cooling cap (`ERR_PIN_COOLING`, 2,500,000 VND) once, in plain words. Entry points: the PIN prompt ("Forgot PIN?") and account settings.
4. **M4 Step-up at confirm**: make `transfer/confirm.tsx` (and the same pattern in `requests/pay.tsx`, `vaults`, `topup`) read `auth_required` from the request result: `pin` asks the PIN; `otp` calls `POST /v1/transactions/challenge` then asks PIN + code; `2fa` asks PIN + TOTP. One shared hook `useTransactionAuth()` returns `{ collect(authRequired) -> {pin, otp?, totp?} | null, prompt }` so screens do not duplicate this. Handle the held-request expiry (`ERR_AUTH_EXPIRED`: tell the user to start again) and the limit errors (ERR_LIMIT_DAILY, ERR_LIMIT_MONTHLY, ERR_TOO_FREQUENT) with their own copy, not the generic one.
5. **M3 TOTP**: `security/totp-enroll.tsx` (password -> show secret/otpauth URI as text and a QR using the existing QR library if one is already a dependency, else text only; no new dependency without owner approval) -> enter first code -> show the 10 recovery codes once with a "I saved them" gate -> status row in account settings (`GET /v1/me/totp`: active, recovery codes left). A recovery code is accepted wherever a TOTP code is asked.
6. **Operator side is not here**: the admin UI for `pin.reset` and `totp.reset` is part of the admin redesign.

## Rules for every step
- The PIN, TOTP code and recovery codes are never logged, never stored in AsyncStorage/SecureStore, never put in a route param or in error text (C-07, C-11). They live only in component state until submitted.
- Money-moving calls keep the persisted intent key (`withIntentKey`) so a retry after a timeout cannot double-submit (C-03).
- Server errors are shown through `describeError`; screens do not invent copy.
- A failed PIN shows remaining attempts only if the API returns them; do not compute locally.
- Accessibility: labels on every input and button (the repo already uses `accessibilityLabel`); numeric keypad; paste allowed for the code.

## Tests
- Jest + React Native Testing Library, following `src/features/**/__tests__` and `app/**/__tests__`: one test file per screen/hook with the API mocked: happy path, each documented error code, double-tap submit sends once, cancel sends nothing.
- `useTransactionAuth` unit tests for the three bands.
- A contract check: a test that fails if an `ERR_*` code listed in `docs/reference/error-codes.md` as user-facing is missing from `errors.ts` (this closes M5 for good).
- Manual device run is NOT possible in CI; mark those cases `HUMAN` in the workbook.

## Acceptance (to be turned into test-cases.json by `hpay-feature` stage 4)
- AC-1 A user with no PIN can set one from account settings and then pay.
- AC-2 A payment attempt with no PIN set leads to set-PIN and returns to the payment.
- AC-3 A user who forgot the PIN completes reset (password, code, new PIN) and sees the cooling notice; a wrong code shows the remaining state, three wrong codes end the attempt.
- AC-4 A payment in the otp band asks for a challenge code and PIN; in the 2fa band asks PIN and TOTP; the confirm payload carries exactly the factors of its band.
- AC-5 A user without TOTP who requests a 2fa payment is sent to enrolment (`ERR_TOTP_NOT_ENROLLED`).
- AC-6 TOTP enrolment shows the recovery codes once and requires the user to confirm they saved them.
- AC-7 Every user-facing backend error code has copy (contract test).
- AC-8 No credential appears in logs, storage or route params (a test greps the mocked logger and storage calls).

## Risks and open questions (for the owner)
- QR rendering needs a library; check what mobile already ships before adding anything.
- Biometric unlock as a factor stays out (legal question on Decision 2345 is open, PLAN G9).
- Real SMS delivery needs the Twilio keys (owner), until then the code only appears in dev responses; the screens must work with the mock sender in dev.
- The redesign will restyle these screens; keep logic in hooks (`useTransactionAuth`, `useSetPin`, `useResetPin`, `useTotpEnrol`) so a restyle touches markup only.

## Effort
About 6 commits. Steps 1, 2, 4 unblock real payments and should go first; step 5 (TOTP) can wait until the first 2fa-band user exists.