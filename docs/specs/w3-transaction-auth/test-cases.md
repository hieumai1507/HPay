---
title: Test cases - transaction authentication and ownership
type: spec
status: proposed
spec: spec.md
---

# Test cases (designed before code)

| Test ID | AC | Type | Priority | Executor | Steps | Expected | Automated as |
|---------|----|------|----------|----------|-------|----------|--------------|
| TC-AUT-001 | AC-1 | Security | P0 | AUTO-API | user B calls confirm and verify with user A's transaction id; and with a random id | both return the same not-found error; nothing about A's transaction is revealed | `test/integration` `TestConfirm_OtherUsersTransactionIsNotFound` |
| TC-AUT-002 | AC-2 | Edge | P0 | AUTO-API | request a transfer whose auth method is `pin` | status `awaiting_auth`, no workflow instance, no ledger entries, balances unchanged | `TestRequest_PinTransferWaitsForAuth` |
| TC-AUT-003 | AC-3 | Negative | P0 | AUTO-API | confirm with a wrong PIN, then with an empty PIN | specific error each time; workflow not started; attempt counter 1 then 2 | `TestConfirm_WrongPinDoesNotStartWorkflow` |
| TC-AUT-004 | AC-4 | Security | P0 | AUTO-API | 5 wrong PINs, then the correct one; then move the lock into the past and retry | locked error even for the correct PIN; works after the lock expires; counter resets on success | `TestPin_LockoutAfterFiveFailures` |
| TC-AUT-005 | AC-5 | Edge (concurrency) | P0 | AUTO-API | correct PIN confirm called 8 times concurrently | exactly one workflow started, transaction completes once, balances moved once | `TestConfirm_ConcurrentStartsWorkflowOnce` |
| TC-AUT-006 | AC-6 | Negative | P1 | AUTO-API | user with no PIN confirms a pin transaction | error telling the user to set a PIN; workflow not started | `TestConfirm_NoPinSet` |
| TC-AUT-007 | AC-7 | Security | P1 | AUTO-API | otp transaction with no OTP, a wrong OTP, a valid OTP; a 2fa transaction | otp: refused, refused, proceeds; 2fa: refused (fail closed) | `TestConfirm_OtpAndTwoFactorBands` |
| TC-AUT-008 | AC-8 | Boundary | P1 | AUTO-API | create an awaiting_auth transaction, make it older than 10 minutes, run the expiry | status `failed`, reason `auth_expired`, no ledger entries | `TestAwaitingAuth_ExpiresWithoutMovingMoney` |
| TC-AUT-009 | AC-9 | Security | P0 | AUTO-API | set PIN with wrong password, right password; change with wrong and right current PIN; weak PINs 000000, 123456, 12345, abcdef; check logs and responses for the PIN | wrong credentials refused; weak refused; the stored value is a hash; the PIN never appears in a response or log | `internal/auth` `TestPin_*` and `TestSetPinEndpoint` |
| TC-AUT-010 | AC-10 | Regression | P0 | AUTO-API | request with auth method none | runs immediately, as before | `TestRequest_AuthNoneRunsImmediately` |
| TC-AUT-011 | AC-12 | Edge | P1 | AUTO-API | create awaiting_auth requests up to the daily limit, then a confirmed one | awaiting requests do not consume the quota | `TestOfferGuard_AwaitingAuthDoesNotConsumeQuota` |
| TC-AUT-012 | AC-11 | Regression | P0 | AUTO-API | full suite; `check-openapi-sync.py` | all pass | whole suite |
| TC-AUT-013 | AC-3 | Boundary | P1 | AUTO-API | the same user confirms the same transaction after it already started | returns the current state; no second workflow | `TestConfirm_AfterStartedReturnsState` |