---
title: Wave 3 slice - transaction authentication (PIN) and ownership of engine confirm/verify
type: spec
status: proposed
last_verified: 2026-11-21
audit: [BE-C2]
related:
  - ../../domain/transfer-and-fees.md
  - ../../domain/identity-and-auth.md
  - ../../adr/0008-server-side-pin.md
  - ../../adr/0014-transaction-pin-gate.md
---

# Wave 3 slice: transaction authentication and ownership (BE-C2)

## Problem (verified in code)
1. `POST /v1/transactions/request` starts the workflow immediately (`engine.Start` in `ExecuteEngineRequest`), so money moves before any authentication. The response carries `auth_required` (pin, otp, 2fa, none) from the offer's auth matrix, but nothing enforces it.
2. `POST /v1/transactions/confirm` and `/verify` accept a PIN, an OTP and a 2FA code and ignore all of them (`transfer/service.go ExecuteEngineConfirm`). They also read any transaction by UUID with no ownership check (IDOR): a signed-in user who knows a transaction id learns another user's reference, amount, fee and status.
3. `users.pin_hash` exists (migration 000001) but no code writes or reads it; there is no endpoint to set or change a PIN.
4. The mobile app sends the literal `123456` for every confirmation.

## Product decisions already taken (user, 2026-11-21)
Server-side PIN: hashed, attempt counter, lockout; confirm must verify it (ADR-0008).

## Scope of this slice
The smallest change that makes the contract true, in the order that keeps money safe:
- **Part A (no design choice): ownership.** `confirm` and `verify` return 404 for a transaction that does not belong to the caller (`initiated_by`). Unknown id and someone else's id are indistinguishable.
- **Part B (the design change): hold the workflow until authenticated.** A request whose `auth_required` is not `none` is created in a new transaction status `awaiting_auth`, without starting the workflow. `confirm` verifies the credential for that method and then starts the workflow. A request with `auth_required = none` starts at once, as today.
- **Part C: PIN lifecycle.** `PUT /v1/me/pin` sets or changes the PIN (set requires the account password; change requires the current PIN); PIN is 6 digits, stored as an argon2id or bcrypt hash (the same function the password uses); 5 wrong attempts lock PIN use for 15 minutes; a successful attempt resets the counter; every failure and lockout is audited.
- **Part D: OTP and 2FA bands.** `otp` uses the existing OTP service (`auth.RequestOTP` / `VerifyOTP`, purpose bound to the transaction); `2fa` has no provider today, so a request that requires `2fa` is refused with a clear error until a provider exists (fail closed). This is stated, not hidden.

## Not in this slice
Rate limiting by account for login (BE-H8), RBAC (BE-H6), changing the offer's auth thresholds (offer-limits spec).

## Acceptance criteria
- **AC-1** Another user's transaction id on `confirm` or `verify` returns 404 and reveals nothing.
- **AC-2** A request whose auth method is `pin`, `otp` or `2fa` does not move money and does not start the workflow until `confirm` succeeds; its status is `awaiting_auth`.
- **AC-3** `confirm` with a wrong or missing PIN for a `pin` transaction fails with a specific error and the workflow is not started; the failure increments the attempt counter.
- **AC-4** After 5 consecutive wrong PINs the account's PIN use is locked for 15 minutes (even a correct PIN is refused during the lock); the lock expires.
- **AC-5** `confirm` with the correct PIN starts the workflow exactly once, even if `confirm` is called twice or concurrently (the second call returns the current state without starting a second workflow).
- **AC-6** A user with no PIN set cannot confirm a `pin` transaction and receives an error that tells them to set a PIN.
- **AC-7** `otp` transactions require a valid OTP bound to that transaction; `2fa` transactions are refused (fail closed) with a clear error.
- **AC-8** An `awaiting_auth` transaction that is never confirmed expires (default 10 minutes) and moves to `failed` with reason `auth_expired`; it never moves money and never blocks the sender's balance.
- **AC-9** `PUT /v1/me/pin`: set with the correct password, change with the correct current PIN, reject weak PINs (not 6 digits, all same digit, sequential), never log or return the PIN.
- **AC-10** A request with `auth_required = none` (an offer configured that way) behaves as today.
- **AC-12** An `awaiting_auth` transaction does not consume the daily or monthly offer quota and does not count toward the interval throttle (found while reading `queries/offer_guard.sql`: `SumUserTxnAmountSince` excludes only `failed` and `reversed`, so unconfirmed requests would otherwise eat the quota).
- **AC-11** The full suite passes; the OpenAPI spec and the sync check are updated; the mobile contract is documented.

## Invariants touched
C-07 (authentication verified on the server), C-08 (ownership predicate), C-03 (confirm is idempotent), C-05 (new status only through the owning function), C-11 (no PIN in logs). Architecture impact: `amend` (transaction state machine gains `awaiting_auth`; the request/confirm flow changes meaning) and `new` credential storage rules. ADR-0014 (proposed).

## Constitution check
| ID | Touched | Compliant | How |
|----|---------|-----------|-----|
| C-03 | yes | yes | confirm starts the workflow once; guarded status transition `awaiting_auth -> processing` |
| C-05 | yes | yes | new status changes only through guarded UPDATEs |
| C-07 | yes | yes after change | credential verified server-side with hash, counter, lockout |
| C-08 | yes | yes | ownership in SQL (`WHERE id = $1 AND initiated_by = $2`) |
| C-11 | yes | yes | PIN hash only; never logged |

## Design consequences found in the code
- `FailTransactionGuarded` and the offer-guard sums enumerate statuses; both must learn `awaiting_auth` (expiry moves it to `failed`; quota sums exclude it).
- The balance is not reserved while `awaiting_auth`: money moves only at MOVE_FUNDS, so the existing soft balance check at request time stays a hint and the hard check runs when the workflow runs, as today.
- The migration adds the enum value (own file, like 000011 and 000016), the PIN counters (000018), and the status writes go through guarded UPDATEs.

## Open questions (assumptions in bold; change them if wrong)
1. PIN format: **6 digits**. Hash: **the same hashing function the password uses** (verify in `auth`), not a new dependency.
2. Lockout: **5 attempts, 15 minutes, per user** (matches the maker-checker OTP lockout shape). Whether to also lock the account after repeated lockouts is a product question.
3. `awaiting_auth` expiry: **10 minutes**.
4. `2fa` band: **refuse (fail closed)** until a provider is chosen; the seed matrix sends amounts above 50,000,000 VND to `2fa`, so large transfers will be refused rather than silently weakened.
5. Existing users have no PIN: the first money movement will require them to set one. Mobile needs a set-PIN screen (the mobile agent is leaving a seam for it).