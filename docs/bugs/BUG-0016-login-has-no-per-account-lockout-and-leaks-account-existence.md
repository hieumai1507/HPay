---
title: Login has no per-account lockout and its timing reveals which accounts exist
id: BUG-0016
status: fixed
severity: high
area: backend/auth
tags: [authn, brute-force, enumeration, timing]
audit: [BE-H8]
constitution: [C-07, C-13]
found: 2026-11-21
fixed: 2026-11-21
commit: pending
regression_test: src/backend/internal/auth/login_lockout_test.go ; src/backend/test/integration/authn/login_lockout_http_test.go
related: [BUG-0012]
---

# BUG-0016: login lockout and constant work

## Symptom
Only a per-IP rate limit (5 per minute) protected `POST /v1/auth/login` and `POST /v1/admin/auth/login`. An attacker with many addresses could guess one account's password without limit. A login for a phone or e-mail that does not exist returned immediately, while a known account spent about 100 ms in argon2id, so response time revealed which accounts exist (for admins too).

## Root cause
`auth.Service.Login` and `auth.AdminService.Login` returned `ErrInvalidCredentials` as soon as the lookup found no row and kept no failure counter. Nothing counted wrong passwords per account.

## Other places with the same flaw
`SetPIN` and `VerifyPIN` already had a counter (BUG-0012). `Register` still answers `ErrPhoneTaken`, which reveals that a phone is registered (open, see below). The per-IP limiter is unchanged and still fails open when Redis fails.

## Fix
Migration 000024 adds `login_failed_attempts` and `login_locked_until` to `users` and `admin_users`. `RecordUserLoginFailure` and `RecordAdminLoginFailure` are single atomic UPDATEs: an expired lock restarts the count, the fifth consecutive failure locks login for 15 minutes. While locked, even the correct password is refused (429 `ERR_LOGIN_LOCKED`). A success resets the counter. Failed attempts and locks are audited (`auth.login_failed`, `auth.login_locked`) without the password. A missing account runs the same argon2id verification (`burnPasswordWork`) so response time no longer reveals existence. `internal/auth/login_guard.go`.

## Regression test
Unit level (real database): five failures lock, correct password refused while locked, success resets, an expired lock allows login, failures audited, unknown account costs about as much as a wrong password, admin login locks the same way, ten parallel wrong attempts still lock. HTTP level: unknown phone and wrong password return the same status and code; the fifth failure and a later correct password both give 429 `ERR_LOGIN_LOCKED`. Mutation checks: removing the dummy hashing, removing the lock check before verifying, and raising the threshold to six each made a test fail.

## How to recognize this again
- Login timing that differs between unknown and known accounts.
- Audit rows `auth.login_failed` growing for one account with no `auth.login_locked`.
- A user reporting they cannot log in for 15 minutes after typing a wrong password several times (expected).

## Docs updated
`domain/identity-and-auth.md` (BR-AUTH-29 to 31), `reference/error-codes.md`, `reference/data-model.md`, OpenAPI, audit report.

## Not done
- A locked-out legitimate user has no self-service unlock (it expires after 15 minutes; an operator can clear the columns). Deliberate lockouts can be abused to deny service to a known phone; the per-IP limit and the audit rows are the only counter-measures.
- `Register` still reveals that a phone is taken (needs a product decision: phone enumeration versus a clear message).
- Login still has no password length cap (hash DoS), the rate limiter still fails open on Redis errors, and `/auth/logout`, `/v1/files` and `/v1/admin/*` have no rate limit.