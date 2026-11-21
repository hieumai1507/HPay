---
title: TOTP second factor for large transactions and a real code-delivery adapter
type: spec
status: done
last_verified: 2026-11-21
adr: ../../adr/0017-totp-second-factor.md
audit: [BE-C2]
---

# Spec: TOTP second factor (2fa band) and a Twilio Verify adapter

## Problem
The offer auth matrix has three bands: `pin` (< 2,000,000), `otp` (2,000,000 to 50,000,000) and `2fa` (>= 50,000,000). The 2fa band is refused by the server (`ErrAuthMethodUnavailable`) because no second factor exists, so a user cannot send 50 million or more. The one-time code port (`auth.Sender`, ADR-0016) has no real provider, so in production neither PIN reset nor the otp band can deliver a code.

## Goals
1. A user can enrol a TOTP authenticator (RFC 6238) and confirm the enrolment with a first code.
2. A transaction in the 2fa band is held in `awaiting_auth` and confirmed with the PIN **and** a valid TOTP code; with no enrolled TOTP it is refused with a clear error telling the user to enrol.
3. Single-use recovery codes (10) for a lost authenticator, plus an operator route to remove a user's TOTP (permission `totp.reset`, superadmin only, reason, audited).
4. A real delivery adapter behind the `Sender` port: Twilio (Programmable Messaging, delivering the code HPay generates), selected by configuration; the mock stays the default outside production; production without a configured provider still fails closed.
5. A factor seam so an on-device biometric factor can be added later without changing the confirm flow.

## Non-goals
- Biometric factor itself (mobile work; see the legal note in `clarify.md`).
- Choosing or integrating a Vietnamese SMS provider.
- Changing the band thresholds or the offer matrix.
- WebAuthn or passkeys.
- E-mail delivery.

## Acceptance criteria
- **AC-1** Given a signed-in user with a PIN, When they start enrolment with their password, Then a new secret is created (not yet active) and the response carries the provisioning URI and the secret ONCE; nothing is returned on later reads.
- **AC-2** Given a pending enrolment, When the user confirms with a valid current code, Then TOTP becomes active and 10 recovery codes are returned once (stored only as hashes).
- **AC-3** Given a pending enrolment, When the code is wrong, Then TOTP stays inactive and the attempt counts toward a lockout (5 wrong in a row lock TOTP use for 15 minutes).
- **AC-4** Given an active TOTP, When a 2fa-band transaction is requested, Then it is held in `awaiting_auth` with `auth_method = 2fa` (no longer refused).
- **AC-5** Given a held 2fa transaction, When confirm carries the right PIN and a valid TOTP code, Then the workflow starts exactly once; with a wrong or missing code it does not start and the failure counts toward the TOTP lockout.
- **AC-6** Given a TOTP code that was already used (same time step), When it is replayed, Then it is refused (a code is accepted once per step).
- **AC-7** Given one step of clock drift (previous or next 30 s step), Then the code is accepted; two steps away it is refused.
- **AC-8** Given a user without active TOTP, When a 2fa-band transaction is requested, Then it is refused with `ERR_TOTP_NOT_ENROLLED` and nothing is held.
- **AC-9** Given a recovery code, When used in place of a TOTP code at confirm, Then it is accepted once and then burnt; the remaining count is reported.
- **AC-10** Given an admin with `totp.reset`, When they remove a user's TOTP with a reason, Then TOTP and recovery codes are removed, audit `totp.reset` exists with admin id and reason, the user is notified; any other admin role gets 403.
- **AC-11** The TOTP secret is stored encrypted at rest (AES-GCM with a key from configuration), never logged, never returned after enrolment, and the audit trail never holds a code or a secret.
- **AC-12** Given two parallel confirms with the same valid code, Then exactly one succeeds.
- **AC-13** Given `SENDER_PROVIDER=twilio_verify` with account settings, Then codes and notices go through the Twilio adapter; given production without a provider, Then the refusing sender remains; given `dev` or `ci`, Then the log-only sender is used; the adapter never logs the code and the HTTP client uses the outbound URL guard (C-10).
- **AC-14** Given a TOTP code in a PIN-band or otp-band transaction, Then it is ignored (the band decides which factors count).

## Invariants (constitution)
C-07 (verified on the server, hashed or encrypted, attempt counter and lockout, audit without secrets), C-08 (operator route by permission; ownership by session), C-09 (no secret or code in responses outside dev), C-10 (the Twilio client uses the outbound guard), C-11 (no secret or code in logs), C-13 (fail closed when no factor or provider), C-14.

## API and schema delta
- `POST /v1/me/totp/enroll` body `{password}` -> `{secret, otpauth_uri}` (once).
- `POST /v1/me/totp/confirm` body `{code}` -> `{status:"totp_enabled", recovery_codes:[...]}` (once).
- `GET /v1/me/totp` -> `{enabled, recovery_codes_left}` (no secret).
- `POST /v1/transactions/confirm` body gains `totp` (a TOTP code or a recovery code).
- `POST /v1/admin/users/:id/totp/reset` body `{reason}` (permission `totp.reset`).
- Migration 000026: `user_totp(user_id PK, secret_enc, enabled, last_step, failed_attempts, locked_until, created_at, activated_at)`, `user_recovery_codes(id, user_id, code_hash, used_at)`.
- Config: `TOTP_ENCRYPTION_KEY` (32 bytes, base64; required when `APP_ENV` is not dev or ci), `SENDER_PROVIDER` (empty or `twilio`), `TWILIO_ACCOUNT_SID`, `TWILIO_API_KEY`, `TWILIO_API_SECRET`, `TWILIO_FROM`.
- Errors: `ERR_TOTP_NOT_ENROLLED` (403), `ERR_TOTP_INVALID` (400), `ERR_TOTP_LOCKED` (429); reuse `ERR_CREDENTIAL_REQUIRED`.
- New permission `totp.reset`; OpenAPI updated.

## Edge-case ids covered
E11 style exhaustion for TOTP; replay and concurrency (AC-6, AC-12).

## Architecture impact: **amend**
A new external dependency (Twilio) behind an existing port, a new secret to protect (encryption key), a new table owned by the auth context and a change to the auth flow. Pages that change: `architecture/building-blocks.md`, `architecture/crosscutting-concepts.md`, `architecture/runtime-views.md`, `architecture/deployment.md`, `domain/identity-and-auth.md`, `domain/transfer-and-fees.md`, `reference/api.md`, `reference/error-codes.md`, `reference/data-model.md`, `reference/configuration.md`, `product/roles-and-permissions.md`. ADR-0017 covers the decision.