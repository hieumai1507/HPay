---
id: BUG-0012
title: Money moves at request time with no authentication, and confirm/verify ignore the credential and expose any transaction by id
status: fixed
severity: blocker
area: backend/transfer
tags: [authz, idor, state-machine, replay]
audit: [BE-C2]
found: 2026-11-21
fixed: 2026-11-21
commit: pending (fix is in the working tree, not committed)
regression_test: src/backend/test/integration/authn/transaction_auth_test.go ; src/backend/internal/auth/pin_test.go
related: [BUG-0001]
---

# BUG-0012: Unauthenticated money movement and IDOR on confirm/verify

(BUG-0004 to BUG-0009 are reserved; the number is out of sequence on purpose.)

## Symptom
1. `POST /v1/transactions/request` started the workflow immediately, so money moved before any authentication, although the response advertised `auth_required` (pin, otp, 2fa).
2. `POST /v1/transactions/confirm` and `/verify` accepted a PIN, OTP and 2FA code and ignored all of them; they returned reference, amount, fee and status of ANY transaction whose id the caller knew (IDOR).
3. There was no PIN endpoint and `users.pin_hash` was never read or written; the mobile app sent the literal `123456`.

## Root cause
`ExecuteEngineRequest` called `engine.Start` unconditionally; `ExecuteEngineConfirm` read the transaction with `GetTransactionByWorkflow(id)` (no owner predicate) and never touched its PIN, OTP or SenderUserID fields; `auth` had no PIN code at all.

## Other places with the same flaw
`/transactions/verify` (same function), and `ExecuteEngineRequest` callers in tests that assumed the request completes by itself (4 test files and one helper) were updated. The legacy `Transfer`/`TransferToUser` path is unmounted dead code and was not changed.

## Fix
ADR-0008 and ADR-0014. A request that needs pin or otp is created as `awaiting_auth` with the method, workflow and service code recorded on the row (migrations 000019, 000020, 000021); the workflow is NOT started and the ledger is untouched. `confirm` locks the row with an ownership predicate in SQL (unknown id and someone else's id both return 404), verifies the credential through `auth.Service.VerifyPIN` (argon2id hash, atomic attempt counter, 5 failures lock PIN use for 15 minutes, a correct PIN is refused during the lock, failures and locks audited without the secret), then moves `awaiting_auth -> processing` with a guarded UPDATE and starts the recorded workflow in the same transaction, so repeated or concurrent confirms start it once. The otp band also needs an OTP bound to the transaction (`/transactions/challenge`). The 2fa band is refused (fail closed) because no provider exists. Unconfirmed requests expire after 10 minutes (`ExpireAwaitingAuth`) and do not count toward offer quota. `PUT/PATCH /v1/me/pin` set and change the PIN (password step-up, then current PIN).

## Regression test
`TestConfirm_OtherUsersTransactionIsNotFound`, `TestRequest_PinTransferWaitsForAuth`, `TestConfirm_WrongPinDoesNotStartWorkflow` (also covers a second confirm), `TestConfirm_ConcurrentStartsWorkflowOnce` (8 goroutines), `TestConfirm_NoPinSet`, `TestConfirm_LockoutRefusesCorrectPin`, `TestRequest_TwoFactorBandIsRefused`, `TestAwaitingAuth_ExpiresWithoutMovingMoney`, `TestOfferGuard_AwaitingAuthDoesNotConsumeQuota`; `TestPin_SetAndChange`, `TestPin_LockoutAfterFiveFailures`, `TestPin_NoPinSet`, `TestPin_FailuresAreAuditedWithoutTheSecret`, `TestValidatePINFormat`. Mutation check on `pin.go`: disabling the lock, the counter reset or the weak-PIN check each made tests fail.

## How to recognize this again
- A transaction in `processing` or `completed` for which no PIN verification was ever recorded.
- `confirm` returning 200 with another account's reference.
- Audit rows `pin.failed` or `pin.locked` bursts for one user.
- A client that sends a fixed PIN string.

## Docs updated
`domain/transfer-and-fees.md`, `domain/identity-and-auth.md`, `reference/data-model.md`, `reference/api.md`, `reference/error-codes.md`, `adr/0014`, `constitution.md` (C-07, C-08), audit report.

## Not done
2FA provider, PIN reset flow (forgotten PIN), per-transaction confirm rate limit beyond the per-user limit, mobile set-PIN screen (the mobile seam exists), the org disbursement path (it has its own maker-checker), and the second layer for the bank callback.