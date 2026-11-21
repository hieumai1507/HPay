---
title: PIN reset - test cases
type: spec
status: approved
last_verified: 2026-11-21
---

# Test cases: PIN reset

Source of the workbook: `test-cases.json` in this folder. Render it with `python -X utf8 .claude/skills/hpay-qa-testcases/scripts/render_feature_workbook.py w4-pin-reset`, which writes `docs/qa/test-cases/w4-pin-reset-testcase.xlsx`. The five catalog workbooks are not changed by this feature.

Executor AUTO-API unless noted. Tests live in `src/backend/internal/auth/pin_reset_test.go` (white-box, real DB) and `src/backend/test/integration/authn/pin_reset_http_test.go` (through the real router).

| TC | AC | Type | Pri | Steps | Expected | Test |
|----|----|------|-----|-------|----------|------|
| TC-PRS-001 | AC-1 | happy | P0 | request with the right password | code sent through the sender, response has no code in `ci` | `TestPINReset_RequestSendsCodeNeverInResponse` |
| TC-PRS-002 | AC-2 | security | P0 | request with a wrong password | `ErrInvalidCredentials`, nothing sent, audit `pin.reset_refused` | `TestPINReset_WrongPasswordSendsNothing` |
| TC-PRS-003 | AC-3 | happy | P0 | request, confirm with the code and a valid PIN | new PIN verifies, old PIN does not, counters cleared, audit `pin.reset` | `TestPINReset_ConfirmReplacesPIN` |
| TC-PRS-004 | AC-3 | replay | P0 | confirm twice with the same code | second fails | `TestPINReset_CodeIsSingleUse` |
| TC-PRS-005 | AC-4 | negative | P0 | three wrong codes, then the correct one | all refused, the PIN unchanged | `TestPINReset_ThreeWrongCodesExhaust` |
| TC-PRS-006 | AC-5 | boundary | P1 | expire the code, confirm | `ErrOTPExpired` | `TestPINReset_ExpiredCodeRefused` |
| TC-PRS-007 | AC-6 | negative | P0 | confirm with weak PINs (111111, 123456, 12ab56, 12345) | `ErrWeakPIN`, the code still works afterwards | `TestPINReset_WeakPINDoesNotConsumeCode` |
| TC-PRS-008 | AC-7 | boundary | P1 | two requests in a row; four requests in an hour | 30 s cooldown, then 3 per hour cap | `TestPINReset_RequestCooldownAndHourlyCap` |
| TC-PRS-009 | AC-8 | edge | P0 | lock the PIN with 5 wrong tries, reset | lock cleared, new PIN works | `TestPINReset_ClearsLockout` |
| TC-PRS-010 | AC-9 | security | P0 | reset, then a PIN transaction above the cooling cap, then a small one | big refused `ErrPINCooling`, small allowed; after the window big allowed | `TestPINReset_CoolingPeriodLimitsLargeTransactions` |
| TC-PRS-011 | AC-10 | happy | P1 | reset | a `pin_reset` notice sent; it contains neither PIN nor code | `TestPINReset_UserIsNotified` |
| TC-PRS-012 | AC-11 | happy | P0 | superadmin clears a PIN with a reason | `pin_hash` NULL, counters cleared, audit with admin id and reason, notice sent | `TestAdminClearPIN_ClearsAndAudits` |
| TC-PRS-013 | AC-12 | security | P0 | every non-superadmin role calls the admin route | 403 for all | `TestAdminClearPIN_OnlySuperadmin` |
| TC-PRS-014 | AC-12 | security | P0 | the permission is in the Go table and the admin app table | tables equal | `TestRolePermissionsMatchFrontend` |
| TC-PRS-015 | AC-13 | negative | P1 | user without a PIN requests a reset | `ErrPINNotSet` | `TestPINReset_NoPINRefused` |
| TC-PRS-016 | AC-14 | concurrency | P0 | 8 parallel confirms, same code | exactly one succeeds | `TestPINReset_ConcurrentConfirmsOneWins` |
| TC-PRS-017 | AC-15 | security | P0 | user A requests; user B confirms with A's code | refused | `TestPINReset_CodeBoundToUser` |
| TC-PRS-018 | AC-1..3 | e2e | P0 | the three HTTP routes through the router, including auth and error codes | 200, 401, 429, 400 mapping | `TestPINResetHTTP_FlowAndErrorCodes` |
| TC-PRS-019 | AC-11 | e2e | P1 | the admin route with an admin token | 200, and a missing reason is 400 | `TestPINResetHTTP_AdminRoute` |
| TC-PRS-020 | C-09 | security | P0 | production-like env uses the refusing sender | request fails closed, no code in any response | `TestOTPSender_ProductionRefuses` |