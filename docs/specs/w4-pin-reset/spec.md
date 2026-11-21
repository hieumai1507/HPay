---
title: Reset a forgotten transaction PIN
type: spec
status: done
last_verified: 2026-11-21
adr: ../../adr/0016-pin-reset.md
audit: [BE-C2]
---

# Spec: reset a forgotten transaction PIN

## Problem
A user who forgot the transaction PIN is locked out of every payment that needs it. `PUT /v1/me/pin` only sets the first PIN (it refuses to change an existing one in practice because `ChangePIN` needs the current PIN). There is no recovery path, so the only remedy is a database edit.

## Goals
1. Self-service reset for a signed-in user: password plus a one-time code, then a new PIN.
2. A fallback for users who cannot receive the code: an operator with the new `pin.reset` permission clears the PIN after verifying identity out of band; the user then sets a new one.
3. A delivery port for the one-time code (mock in dev and CI), so a real SMS or e-mail provider can be added without touching the flow.
4. Abuse resistance: reuse the PIN attempt counter and lockout, cooldowns, a cooling period after reset, audit rows, and a notification to the user.

## Non-goals
- Choosing or integrating a real SMS or e-mail provider (separate ADR with 2FA).
- Resetting the login password (separate feature).
- Reset for a user who is not signed in.
- Biometric or device-bound recovery.
- Self-service unlock of a login lockout (BUG-0016 follow-up).

## Acceptance criteria
- **AC-1** Given a signed-in user with a PIN, When they request a reset with the correct password, Then a code is sent through the delivery port and the response never contains the code (outside `APP_ENV=dev`).
- **AC-2** Given a request with a wrong password, Then it is refused (401 `ERR_INVALID_CREDENTIALS`), nothing is sent and the failure is audited.
- **AC-3** Given a valid code and a valid new PIN, When the user confirms, Then the PIN hash is replaced, the PIN failure counter and lock are cleared, the code is consumed and cannot be used again, and a `pin.reset` audit row exists.
- **AC-4** Given a wrong code, Then it is refused with `ERR_OTP_*`, the third wrong attempt exhausts the code (a later correct code fails), and nothing changes.
- **AC-5** Given an expired code, Then it is refused.
- **AC-6** Given a weak new PIN (not 6 digits, same digit, a run), Then it is refused (`ERR_WEAK_PIN`) and the code is NOT consumed.
- **AC-7** Given a second request within 30 seconds, Then it is refused (429 `ERR_OTP_COOLDOWN`); given more than 3 requests per hour, Then it is refused (429).
- **AC-8** Given a PIN that is locked after 5 wrong attempts, When the user completes a reset, Then the lock is cleared (this is the main recovery path).
- **AC-9** Given a completed reset, Then for the next 24 hours a transaction above 2,500,000 VND needing a PIN is refused with `ERR_PIN_COOLING` (the owner can change the numbers), and ordinary smaller transactions work.
- **AC-10** Given a completed reset, Then the user is notified through the delivery port (`pin_reset` notice) and the notice never contains the PIN or the code.
- **AC-11** Given an admin with `pin.reset`, When they clear a user's PIN with a reason, Then `pin_hash` is NULL, counters are cleared, an audit row with the admin id and the reason exists, the user is notified, and the cooling period applies after the user sets a new PIN.
- **AC-12** Given an admin without `pin.reset` (viewer, kyc_reviewer, ops, risk_officer), Then the endpoint returns 403.
- **AC-13** Given a user with no PIN set, Then a reset request is refused with `ERR_PIN_NOT_SET` (use set-PIN instead).
- **AC-14** Given two concurrent confirms with the same code, Then exactly one succeeds.
- **AC-15** A pending reset code of user A can never be used by user B (the code is bound to the user and the purpose `pin_reset`).

## Invariants (constitution)
C-07 (credentials verified on the server, hashed, attempt counters, audit), C-08 (admin permission checked on the server; ownership by session), C-09 (code in a response only in dev), C-11 (no PIN or code in logs or audit), C-14 (test first, knowledge base updated).

## API and schema delta
- `POST /v1/me/pin/reset/request` body `{password}` -> 200 `{status:"code_sent", expires_in}` (dev adds `otp_dev`).
- `POST /v1/me/pin/reset/confirm` body `{code, new_pin}` -> 200 `{status:"pin_reset"}`.
- `POST /v1/admin/users/:id/pin/reset` body `{reason}` (permission `pin.reset`) -> 200.
- New permission `pin.reset` (superadmin only; the Go table and the admin app table must match, the RBAC drift test enforces it).
- Migration 000025: `users.pin_reset_at TIMESTAMPTZ`; `otp_challenges` already carries `purpose`.
- Errors: `ERR_PIN_COOLING` (403), reuse `ERR_OTP_*`, `ERR_WEAK_PIN`, `ERR_PIN_NOT_SET`, `ERR_RATE_LIMITED`.
- OpenAPI updated for the three routes.

## Edge-case ids covered
E11 (OTP exhaustion) extends to a new purpose; replay and concurrency covered by AC-14.

## Architecture impact: **amend**
A new outbound port (`OTPSender`) is a new dependency boundary and the auth module gains a flow. Pages that change: `architecture/building-blocks.md` (auth block gains the notifier port), `architecture/crosscutting-concepts.md` (OTP delivery), `architecture/runtime-views.md` (new sequence), `domain/identity-and-auth.md`, `reference/api.md`, `reference/error-codes.md`, `reference/data-model.md`, `reference/configuration.md` (sender selection), `product/roles-and-permissions.md`. ADR-0016 (proposed) covers the decision.