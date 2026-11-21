---
title: ADR 0008 Server-side PIN
type: decision
status: accepted (implemented by ADR-0014; reset flow by ADR-0016)
last_verified: 2026-11-21
---

# ADR 0008: Server-side PIN

Status: Accepted. Implemented in the working tree (not yet committed) by ADR-0014: PIN set/change endpoints, argon2id hash, attempt counter and lockout, confirm verifies it. Not implemented: PIN reset, 2FA provider, admin unlock.

## Context
The offer auth matrix says transactions can require `pin`, `otp` or `2fa`, but the server does not enforce it: `/transactions/confirm` and `/transactions/verify` ignore PIN and OTP and have an IDOR (audit BE-C2); `ExecuteEngineRequest` starts the workflow immediately; there is no PIN route at all. The mobile app sends a hard-coded `123456` and compares the app lock PIN on the client. Authentication of money movement is void at both ends.

## Decision
Implement a real server-side PIN:
- Store only a salted hash (argon2id, like passwords). Set and change through authenticated endpoints with step-up (OTP or current PIN).
- Count failed attempts per user (not per IP), lock after N failures for a cooling period, reset on success, audit every failure and lock.
- The workflow stops at an AUTH step until `confirm` presents a valid PIN (or OTP) for the exact transaction; `confirm` verifies `initiated_by == caller` (404 otherwise), is idempotent, and expires.
- The client never holds or compares the PIN; biometrics unlock a local key only and the PIN is still verified by the server for money movement.
- Rate limit confirm per user and per transaction.

## Consequences
- Closes BE-C2 and the mobile hard-coded PIN findings once implemented.
- Needs a migration (PIN hash, attempts, locked_until), engine changes (AUTH step or state before MOVE_FUNDS), openapi and mobile changes, and tests for wrong PIN, lockout, IDOR and replay.
- Until then the system must not be exposed to real money.
- Lockout per user creates a denial-of-service risk against a victim; mitigate with a cooling period and admin unlock.